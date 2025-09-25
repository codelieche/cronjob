package runner

import (
	"os"
	"testing"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// createSecurityTestTask 创建安全测试用的Task对象
func createSecurityTestTask(command, args string) *core.Task {
	return &core.Task{
		ID:      uuid.New(),
		Command: command,
		Args:    args,
		Timeout: 30, // 默认30秒超时
	}
}

// TestCommandRunner_Security 测试安全检查功能
func TestCommandRunner_Security(t *testing.T) {
	// 测试正常命令
	runner := NewCommandRunner()
	task := createSecurityTestTask("echo", "hello world")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("正常命令应该通过安全检查: %v", err)
	}

	// 测试ls命令
	task = createSecurityTestTask("ls", "-la")
	err = runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("ls命令应该通过安全检查: %v", err)
	}

	// 测试bash命令
	task = createSecurityTestTask("bash", "-c 'echo hello'")
	err = runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("bash命令应该通过安全检查: %v", err)
	}
}

// TestCommandRunner_SecurityDisabled 测试禁用安全检查
func TestCommandRunner_SecurityDisabled(t *testing.T) {
	// 设置环境变量禁用安全检查
	os.Setenv("COMMAND_SECURITY_DISABLED", "true")
	defer os.Unsetenv("COMMAND_SECURITY_DISABLED")

	runner := NewCommandRunner()

	// 即使是不安全的命令也应该通过
	task := createSecurityTestTask("rm", "-rf /")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("禁用安全检查后，危险命令应该通过: %v", err)
	}
}

// TestCommandRunner_SecurityBlocked 测试被阻止的命令
func TestCommandRunner_SecurityBlocked(t *testing.T) {
	// 设置环境变量添加黑名单
	os.Setenv("COMMAND_BLOCKED", "rm,dd,format")
	defer os.Unsetenv("COMMAND_BLOCKED")

	runner := NewCommandRunner()

	// 测试被阻止的命令
	task := createSecurityTestTask("rm", "-rf /tmp/test")
	err := runner.ParseArgs(task)
	if err == nil {
		t.Fatal("rm命令应该被安全检查阻止")
	}

	if !contains(err.Error(), "被禁止执行") {
		t.Fatalf("错误信息应该包含'被禁止执行'，实际: %v", err)
	}

	// 测试dd命令
	task = createSecurityTestTask("dd", "if=/dev/zero of=/dev/sda")
	err = runner.ParseArgs(task)
	if err == nil {
		t.Fatal("dd命令应该被安全检查阻止")
	}
}

// TestCommandRunner_SecurityAllowed 测试白名单功能
func TestCommandRunner_SecurityAllowed(t *testing.T) {
	// 设置环境变量添加白名单
	os.Setenv("COMMAND_ALLOWED", "echo,ls,cat")
	defer os.Unsetenv("COMMAND_ALLOWED")

	runner := NewCommandRunner()

	// 测试白名单中的命令
	task := createSecurityTestTask("echo", "hello")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("白名单中的echo命令应该通过: %v", err)
	}

	task = createSecurityTestTask("ls", "-la")
	err = runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("白名单中的ls命令应该通过: %v", err)
	}

	// 测试不在白名单中的命令
	task = createSecurityTestTask("rm", "test.txt")
	err = runner.ParseArgs(task)
	if err == nil {
		t.Fatal("不在白名单中的rm命令应该被阻止")
	}

	if !contains(err.Error(), "不在允许列表中") {
		t.Fatalf("错误信息应该包含'不在允许列表中'，实际: %v", err)
	}
}

// TestExtractBaseCommand 测试提取基础命令功能
func TestExtractBaseCommand(t *testing.T) {
	runner := NewCommandRunner()

	tests := []struct {
		input    string
		expected string
	}{
		{"echo hello", "echo"},
		{"ls -la", "ls"},
		{"bash -c 'echo hello'", "bash"},
		{"  echo   hello  ", "echo"},
		{"", ""},
		{"   ", ""},
		{"echo", "echo"},
	}

	for _, test := range tests {
		result := runner.extractBaseCommand(test.input)
		if result != test.expected {
			t.Errorf("extractBaseCommand(%q) = %q, 期望 %q", test.input, result, test.expected)
		}
	}
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

// containsSubstring 简单的子字符串检查
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
