package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRunner_DefaultStatusValidation(t *testing.T) {
	t.Run("默认行为-2xx成功", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK) // 200
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		runner := NewHTTPRunner()
		config := HTTPConfig{
			URL:    server.URL,
			Method: "GET",
			// 不设置 ExpectedStatus，使用默认行为
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 100)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, result.Status)
	})

	t.Run("默认行为-3xx成功", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusMovedPermanently) // 301
		}))
		defer server.Close()

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

		ctx := context.Background()
		logChan := make(chan string, 100)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err) // 3xx 也算成功
		assert.Equal(t, core.StatusSuccess, result.Status)
	})

	t.Run("默认行为-4xx失败", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

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

		ctx := context.Background()
		logChan := make(chan string, 100)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.Error(t, err)
		assert.Equal(t, core.StatusFailed, result.Status)
		assert.Contains(t, err.Error(), "状态码 404")
	})

	t.Run("默认行为-5xx失败", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError) // 500
			w.Write([]byte(`{"error": "server error"}`))
		}))
		defer server.Close()

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

		ctx := context.Background()
		logChan := make(chan string, 100)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.Error(t, err)
		assert.Equal(t, core.StatusFailed, result.Status)
		assert.Contains(t, err.Error(), "状态码 500")
	})
}
