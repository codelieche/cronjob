package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestScriptRunner_ParseArgs 测试参数解析
func TestScriptRunner_ParseArgs(t *testing.T) {
	t.Run("基本配置-文件模式", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "file",
			File:     "/var/scripts/test.py",
			Args:     []string{"arg1", "arg2"},
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 30,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)
		assert.Equal(t, "python", runner.config.Language)
		assert.Equal(t, "file", runner.config.Type)
		assert.Equal(t, "/var/scripts/test.py", runner.config.File)
		assert.Equal(t, []string{"arg1", "arg2"}, runner.config.Args)
	})

	t.Run("基本配置-内联模式", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     "print('Hello, World!')",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 30,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)
		assert.Equal(t, "python", runner.config.Language)
		assert.Equal(t, "inline", runner.config.Type)
		assert.Equal(t, "print('Hello, World!')", runner.config.Code)
	})

	t.Run("语言名称标准化", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"python", "python"},
			{"Python", "python"},
			{"PYTHON", "python"},
			{"nodejs", "nodejs"},
			{"node", "nodejs"},
			{"javascript", "nodejs"},
			{"js", "nodejs"},
			{"shell", "shell"},
			{"bash", "shell"},
			{"sh", "shell"},
		}

		for _, tc := range testCases {
			runner := NewScriptRunner()
			config := ScriptConfig{
				Language: tc.input,
				Type:     "inline",
				Code:     "echo test",
			}
			argsBytes, _ := json.Marshal(config)

			task := &core.Task{
				ID:   uuid.New(),
				Args: string(argsBytes),
			}

			err := runner.ParseArgs(task)
			assert.NoError(t, err, "Language: %s", tc.input)
			assert.Equal(t, tc.expected, runner.config.Language, "Language: %s", tc.input)
		}
	})
}

// TestScriptRunner_ParseArgs_Validation 测试配置验证
func TestScriptRunner_ParseArgs_Validation(t *testing.T) {
	t.Run("不支持的语言", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "java",
			Type:     "file",
			File:     "/test.java",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "不支持的语言")
	})

	t.Run("文件模式-缺少文件路径", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "file",
			File:     "",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file 字段不能为空")
	})

	t.Run("内联模式-缺少代码内容", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     "",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "code 字段不能为空")
	})

	t.Run("内联模式-代码长度超限", func(t *testing.T) {
		runner := NewScriptRunner()

		// 创建一个超过10KB的代码
		largeCode := make([]byte, maxInlineCodeSize+1)
		for i := range largeCode {
			largeCode[i] = 'a'
		}

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     string(largeCode),
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "内联代码长度超过限制")
	})

	t.Run("无效的类型", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "invalid",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:   uuid.New(),
			Args: string(argsBytes),
		}

		err := runner.ParseArgs(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type 必须是 'file' 或 'inline'")
	})
}

// TestScriptRunner_Execute_Inline 测试内联模式执行
func TestScriptRunner_Execute_Inline(t *testing.T) {
	t.Run("Python-打印Hello", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     "print('Hello from Python')",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "Hello from Python")
	})

	t.Run("Shell-简单命令", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "shell",
			Type:     "inline",
			Code:     "echo 'Hello from Shell'",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "Hello from Shell")
	})

	t.Run("Node.js-打印信息", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "nodejs",
			Type:     "inline",
			Code:     "console.log('Hello from Node.js');",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "Hello from Node.js")
	})
}

// TestScriptRunner_Execute_WithArgs 测试参数传递
func TestScriptRunner_Execute_WithArgs(t *testing.T) {
	t.Run("Python-接收参数", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     "import sys\nprint('Args:', ' '.join(sys.argv[1:]))",
			Args:     []string{"arg1", "arg2", "arg3"},
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "arg1 arg2 arg3")
	})

	t.Run("Shell-接收参数", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "shell",
			Type:     "inline",
			Code:     "echo \"Args: $@\"",
			Args:     []string{"test1", "test2"},
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "test1 test2")
	})
}

// TestScriptRunner_Execute_File 测试文件模式执行
func TestScriptRunner_Execute_File(t *testing.T) {
	// 创建临时测试脚本文件
	tmpDir := t.TempDir()

	// 临时添加到白名单（测试用）
	oldAllowedDirs := allowedScriptDirs
	allowedScriptDirs = append(allowedScriptDirs, tmpDir)
	defer func() { allowedScriptDirs = oldAllowedDirs }()

	t.Run("Python文件", func(t *testing.T) {
		// 创建Python脚本文件
		scriptPath := filepath.Join(tmpDir, "test.py")
		scriptContent := "print('Hello from file')"
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
		assert.NoError(t, err)

		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "file",
			File:     scriptPath,
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err = runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "Hello from file")
	})

	t.Run("Shell文件", func(t *testing.T) {
		// 创建Shell脚本文件
		scriptPath := filepath.Join(tmpDir, "test.sh")
		scriptContent := "#!/bin/bash\necho 'Shell script executed'"
		err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
		assert.NoError(t, err)

		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "shell",
			Type:     "file",
			File:     scriptPath,
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err = runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.Status)
		assert.Contains(t, result.Output, "Shell script executed")
	})

	t.Run("文件不存在", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "file",
			File:     filepath.Join(tmpDir, "notexist.py"),
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.Status)
		assert.Contains(t, err.Error(), "脚本文件不存在")
	})
}

// TestScriptRunner_Execute_Timeout 测试超时控制
func TestScriptRunner_Execute_Timeout(t *testing.T) {
	runner := NewScriptRunner()

	// Python脚本：睡眠5秒
	config := ScriptConfig{
		Language: "python",
		Type:     "inline",
		Code:     "import time\ntime.sleep(5)\nprint('Done')",
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:      uuid.New(),
		Args:    string(argsBytes),
		Timeout: 1, // 1秒超时
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	ctx := context.Background()
	logChan := make(chan string, 10)
	defer close(logChan)

	result, err := runner.Execute(ctx, logChan)
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "超时")
}

// TestScriptRunner_Execute_Error 测试错误处理
func TestScriptRunner_Execute_Error(t *testing.T) {
	t.Run("Python语法错误", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "python",
			Type:     "inline",
			Code:     "print('missing quote",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.Status)
	})

	t.Run("Shell命令失败", func(t *testing.T) {
		runner := NewScriptRunner()

		config := ScriptConfig{
			Language: "shell",
			Type:     "inline",
			Code:     "exit 1",
		}
		argsBytes, _ := json.Marshal(config)

		task := &core.Task{
			ID:      uuid.New(),
			Args:    string(argsBytes),
			Timeout: 10,
		}

		err := runner.ParseArgs(task)
		assert.NoError(t, err)

		ctx := context.Background()
		logChan := make(chan string, 10)
		defer close(logChan)

		result, err := runner.Execute(ctx, logChan)
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.Status)
		assert.Equal(t, int64(1), result.ExitCode)
	})
}

// TestScriptRunner_TempFileCleanup 测试临时文件清理
func TestScriptRunner_TempFileCleanup(t *testing.T) {
	runner := NewScriptRunner()

	config := ScriptConfig{
		Language: "python",
		Type:     "inline",
		Code:     "print('test')",
	}
	argsBytes, _ := json.Marshal(config)

	task := &core.Task{
		ID:      uuid.New(),
		Args:    string(argsBytes),
		Timeout: 10,
	}

	err := runner.ParseArgs(task)
	assert.NoError(t, err)

	ctx := context.Background()
	logChan := make(chan string, 10)
	defer close(logChan)

	// 执行任务
	result, err := runner.Execute(ctx, logChan)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 记录临时文件路径
	tempFile := runner.tempFile
	assert.NotEmpty(t, tempFile)

	// 执行清理
	err = runner.Cleanup()
	assert.NoError(t, err)

	// 验证临时文件已删除
	_, err = os.Stat(tempFile)
	assert.True(t, os.IsNotExist(err))
}

// TestScriptRunner_GetInterpreter 测试解释器路径获取
func TestScriptRunner_GetInterpreter(t *testing.T) {
	testCases := []struct {
		language string
		custom   string
		expected string
	}{
		{"python", "", "python3"},
		{"python", "/usr/bin/python3.9", "/usr/bin/python3.9"},
		{"nodejs", "", "node"},
		{"nodejs", "/usr/local/bin/node", "/usr/local/bin/node"},
		{"shell", "", "/bin/bash"},
	}

	for _, tc := range testCases {
		runner := NewScriptRunner()
		runner.config = &ScriptConfig{
			Language:    tc.language,
			Interpreter: tc.custom,
		}

		interpreter := runner.getInterpreter()
		assert.Equal(t, tc.expected, interpreter,
			"Language: %s, Custom: %s", tc.language, tc.custom)
	}
}

// TestScriptRunner_GetScriptExtension 测试脚本扩展名获取
func TestScriptRunner_GetScriptExtension(t *testing.T) {
	testCases := []struct {
		language string
		expected string
	}{
		{"python", ".py"},
		{"nodejs", ".js"},
		{"shell", ".sh"},
	}

	for _, tc := range testCases {
		runner := NewScriptRunner()
		runner.config = &ScriptConfig{Language: tc.language}

		ext := runner.getScriptExtension()
		assert.Equal(t, tc.expected, ext, "Language: %s", tc.language)
	}
}

// TestScriptRunner_Status 测试状态获取
func TestScriptRunner_Status(t *testing.T) {
	runner := NewScriptRunner()

	// 初始状态
	status := runner.GetStatus()
	assert.Equal(t, core.StatusPending, status)

	// 模拟执行中
	runner.mutex.Lock()
	runner.status = core.StatusRunning
	runner.mutex.Unlock()

	status = runner.GetStatus()
	assert.Equal(t, core.StatusRunning, status)

	// 模拟成功
	runner.mutex.Lock()
	runner.status = core.StatusSuccess
	runner.mutex.Unlock()

	status = runner.GetStatus()
	assert.Equal(t, core.StatusSuccess, status)
}
