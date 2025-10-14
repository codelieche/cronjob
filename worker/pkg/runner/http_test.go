package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRunner_ParseArgs(t *testing.T) {
	runner := NewHTTPRunner()

	// 测试1: 基本配置
	t.Run("基本配置", func(t *testing.T) {
		config := HTTPConfig{
			URL:    "https://api.example.com/test",
			Method: "GET",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 30, // 超时在Task层面设置
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)
		assert.Equal(t, "GET", runner.config.Method)
		assert.Equal(t, 30*time.Second, runner.client.Timeout) // 超时使用Task的配置
	})

	// 测试2: URL验证
	t.Run("无效URL-空", func(t *testing.T) {
		config := HTTPConfig{
			URL: "",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL不能为空")
	})

	t.Run("无效URL-协议", func(t *testing.T) {
		config := HTTPConfig{
			URL: "ftp://invalid-url",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP或HTTPS")
	})

	// 测试3: 方法验证（只支持4种）
	t.Run("不支持的方法", func(t *testing.T) {
		config := HTTPConfig{
			URL:    "https://api.example.com/test",
			Method: "PATCH", // v2.0 不支持
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "不支持的HTTP方法")
	})
}

func TestHTTPRunner_Execute_GET_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "success"}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "GET",
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:   uuid.New(),
		Args: string(argsBytes),
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, result.Status)
	assert.Contains(t, result.Output, "ok")
	assert.Equal(t, 200, result.ExitCode)
}

func TestHTTPRunner_Execute_POST_WithBody(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 验证Body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "张三", body["name"])

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 123, "status": "created"}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "POST",
		Body: map[string]interface{}{
			"name": "张三",
			"age":  25,
		},
		ExpectedStatus: []int{201},
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:   uuid.New(),
		Args: string(argsBytes),
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, result.Status)
	assert.Contains(t, result.Output, "created")
	assert.Equal(t, 201, result.ExitCode)
}

func TestHTTPRunner_Execute_WithHeaders(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证Headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "my-app", r.Header.Get("X-App-Name"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"authenticated": true}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"X-App-Name":    "my-app",
		},
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:   uuid.New(),
		Args: string(argsBytes),
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, result.Status)
}

func TestHTTPRunner_Execute_WithQuery(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证Query参数
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		assert.Equal(t, "20", r.URL.Query().Get("size"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"page": 1, "size": 20}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "GET",
		Query: map[string]string{
			"page": "1",
			"size": "20",
		},
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:   uuid.New(),
		Args: string(argsBytes),
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, result.Status)
}

// 注意：重试功能已移到Task层面处理，HTTPRunner不再内部重试

func TestHTTPRunner_Execute_Timeout(t *testing.T) {
	// 创建测试服务器（延迟响应）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建Runner（超时时间1秒，在Task层面设置）
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "GET",
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:      uuid.New(),
		Args:    string(argsBytes),
		Timeout: 1, // 超时在Task层面设置
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.Error(t, err)
	assert.Equal(t, core.StatusFailed, result.Status)
}

func TestHTTPRunner_Execute_StatusCodeValidation(t *testing.T) {
	// 创建测试服务器（返回404）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:            server.URL,
		Method:         "GET",
		ExpectedStatus: []int{200}, // 期望200，但实际是404
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:   uuid.New(),
		Args: string(argsBytes),
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.Error(t, err)
	assert.Equal(t, core.StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "状态码不符合预期")
}

func TestHTTPRunner_VariableReplacement(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证环境变量已被替换
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		assert.Equal(t, "test-value", r.URL.Query().Get("key"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// 创建Runner
	runner := NewHTTPRunner()
	config := HTTPConfig{
		URL:    server.URL,
		Method: "GET",
		Headers: map[string]string{
			"Authorization": "Bearer ${TOKEN}",
		},
		Query: map[string]string{
			"key": "${VALUE}",
		},
	}
	argsBytes, _ := json.Marshal(config)

	// 设置环境变量
	metadataBytes, _ := json.Marshal(map[string]interface{}{
		"environment": map[string]string{
			"TOKEN": "my-secret-token",
			"VALUE": "test-value",
		},
	})

	task := &core.Task{
		ID:       uuid.New(),
		Args:     string(argsBytes),
		Metadata: metadataBytes,
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	// 执行请求
	ctx := context.Background()
	logChan := make(chan string, 100)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, result.Status)
}
