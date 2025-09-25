package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// createTestTask 创建测试用的Task对象
func createTestTask(command, args string, timeout ...int) *core.Task {
	timeoutSec := 30 // 默认30秒超时
	if len(timeout) > 0 {
		timeoutSec = timeout[0]
	}

	return &core.Task{
		ID:      uuid.New(),
		Command: command,
		Args:    args,
		Timeout: timeoutSec,
	}
}

// TestCommandRunner_ParseArgs 测试参数解析功能
func TestCommandRunner_ParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        string
		expectError bool
		description string
	}{
		{
			name:        "基本命令",
			command:     "ls -la",
			args:        "",
			expectError: false,
			description: "只有command，没有args",
		},
		{
			name:        "带参数命令",
			command:     "echo",
			args:        "hello world",
			expectError: false,
			description: "command和args拼接",
		},
		{
			name:        "复杂命令",
			command:     "ping 8.8.8.8 -c 3 && echo `date` >> ./logs/ping.log && echo `date`",
			args:        "",
			expectError: false,
			description: "包含管道和重定向的复杂命令",
		},
		{
			name:        "空命令",
			command:     "",
			args:        "",
			expectError: true,
			description: "空命令应该返回错误",
		},
		{
			name:        "只有空格的命令",
			command:     "   ",
			args:        "",
			expectError: true,
			description: "只有空格的命令应该返回错误",
		},
		{
			name:        "JSON格式args",
			command:     "echo",
			args:        "[]",
			expectError: false,
			description: "JSON格式的空参数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()
			task := createTestTask(tt.command, tt.args)
			err := runner.ParseArgs(task)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望返回错误，但没有错误")
				}
			} else {
				if err != nil {
					t.Errorf("不期望返回错误，但得到: %v", err)
				}
			}
		})
	}
}

// TestCommandRunner_BasicExecution 测试基本命令执行
func TestCommandRunner_BasicExecution(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           string
		expectedStatus core.Status
		expectedOutput string
		description    string
	}{
		{
			name:           "简单echo命令",
			command:        "echo",
			args:           "hello world",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "hello world\n",
			description:    "基本的echo命令执行",
		},
		{
			name:           "date命令",
			command:        "date",
			args:           "",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "", // 输出内容会变化，只检查状态
			description:    "date命令执行",
		},
		{
			name:           "pwd命令",
			command:        "pwd",
			args:           "",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "", // 输出内容会变化，只检查状态
			description:    "pwd命令执行",
		},
		{
			name:           "whoami命令",
			command:        "whoami",
			args:           "",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "", // 输出内容会变化，只检查状态
			description:    "whoami命令执行",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 解析参数
			task := createTestTask(tt.command, tt.args)
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 执行命令
			ctx := context.Background()
			logChan := make(chan string, 10)
			defer close(logChan)

			result, err := runner.Execute(ctx, logChan)
			if err != nil {
				t.Fatalf("执行命令失败: %v", err)
			}

			// 验证结果
			if result.Status != tt.expectedStatus {
				t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
			}

			if result.ExitCode != 0 {
				t.Errorf("期望退出码为0，实际为 %d", result.ExitCode)
			}

			if tt.expectedOutput != "" && result.Output != tt.expectedOutput {
				t.Errorf("期望输出为 %q，实际为 %q", tt.expectedOutput, result.Output)
			}

			// 验证执行时间
			if result.Duration <= 0 {
				t.Error("执行时间应该大于0")
			}

			// 验证时间字段
			if result.StartTime.IsZero() {
				t.Error("开始时间不应该为零值")
			}
			if result.EndTime.IsZero() {
				t.Error("结束时间不应该为零值")
			}
			if result.EndTime.Before(result.StartTime) {
				t.Error("结束时间不应该早于开始时间")
			}
		})
	}
}

// TestCommandRunner_TimeoutHandling 测试超时处理
func TestCommandRunner_TimeoutHandling(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		timeout        time.Duration
		expectedStatus core.Status
		description    string
	}{
		{
			name:           "短超时时间",
			command:        "sleep 5",
			timeout:        1 * time.Second,
			expectedStatus: core.StatusTimeout,
			description:    "1秒超时，5秒sleep应该超时",
		},
		{
			name:           "中等超时时间",
			command:        "sleep 3",
			timeout:        2 * time.Second,
			expectedStatus: core.StatusTimeout,
			description:    "2秒超时，3秒sleep应该超时",
		},
		{
			name:           "长超时时间",
			command:        "sleep 1",
			timeout:        5 * time.Second,
			expectedStatus: core.StatusSuccess,
			description:    "5秒超时，1秒sleep应该成功",
		},
		{
			name:           "无超时限制",
			command:        "sleep 1",
			timeout:        0,
			expectedStatus: core.StatusSuccess,
			description:    "无超时限制，1秒sleep应该成功",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 创建Task对象
			task := &core.Task{
				ID:      uuid.New(),
				Command: tt.command,
				Args:    "",
				Timeout: int(tt.timeout.Seconds()),
			}

			// 解析参数
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 执行命令
			ctx := context.Background()
			logChan := make(chan string, 10)
			defer close(logChan)

			startTime := time.Now()
			result, err := runner.Execute(ctx, logChan)
			actualDuration := time.Since(startTime)

			if err != nil {
				t.Fatalf("执行命令失败: %v", err)
			}

			// 验证状态
			if result.Status != tt.expectedStatus {
				t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
			}

			// 验证超时情况
			if tt.expectedStatus == core.StatusTimeout {
				// 检查错误消息
				if result.Error == "" {
					t.Error("超时时错误消息不应该为空")
				}

				// 检查是否包含超时信息
				if !strings.Contains(result.Error, "超时") {
					t.Errorf("错误消息应该包含超时信息，实际为: %s", result.Error)
				}

				// 验证实际执行时间应该在超时时间附近
				expectedMinDuration := tt.timeout.Milliseconds() - 100  // 允许100ms误差
				expectedMaxDuration := tt.timeout.Milliseconds() + 1000 // 允许1s误差

				if actualDuration.Milliseconds() < expectedMinDuration {
					t.Errorf("实际执行时间 %dms 小于期望的最小时间 %dms",
						actualDuration.Milliseconds(), expectedMinDuration)
				}
				if actualDuration.Milliseconds() > expectedMaxDuration {
					t.Errorf("实际执行时间 %dms 大于期望的最大时间 %dms",
						actualDuration.Milliseconds(), expectedMaxDuration)
				}
			} else {
				// 成功情况，验证没有错误
				if result.Error != "" {
					t.Errorf("成功执行时不应该有错误，但得到: %s", result.Error)
				}
			}

			t.Logf("执行时间: %v, 状态: %s, 错误: %s",
				actualDuration, result.Status, result.Error)
		})
	}
}

// TestCommandRunner_StopFunctionality 测试停止功能
func TestCommandRunner_StopFunctionality(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		stopDelay      time.Duration
		expectedStatus core.Status
		description    string
	}{
		{
			name:           "快速停止",
			command:        "sleep 10",
			stopDelay:      200 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "200ms后停止10秒sleep",
		},
		{
			name:           "中等延迟停止",
			command:        "sleep 10",
			stopDelay:      500 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "500ms后停止10秒sleep",
		},
		{
			name:           "while循环停止",
			command:        "while true; do sleep 1; done",
			stopDelay:      300 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "停止无限循环命令",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 解析参数
			task := createTestTask(tt.command, "")
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 在goroutine中执行命令
			ctx := context.Background()
			done := make(chan *core.Result, 1)
			execErr := make(chan error, 1)

			go func() {
				logChan := make(chan string, 10)
				defer close(logChan)
				result, err := runner.Execute(ctx, logChan)
				if err != nil {
					execErr <- err
					return
				}
				done <- result
			}()

			// 等待一下确保命令开始执行
			time.Sleep(100 * time.Millisecond)

			// 检查任务是否在运行
			if runner.GetStatus() != core.StatusRunning {
				t.Fatalf("任务应该正在运行，当前状态: %s", runner.GetStatus())
			}

			// 等待指定时间后停止
			time.Sleep(tt.stopDelay)

			// 停止命令
			err = runner.Stop()
			if err != nil {
				t.Fatalf("停止命令失败: %v", err)
			}

			// 等待执行完成
			select {
			case result := <-done:
				// 验证结果
				if result.Status != tt.expectedStatus {
					t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
				}

				// 检查错误消息
				if result.Error == "" {
					t.Error("停止时错误消息不应该为空")
				}

				// 检查错误消息是否包含停止信息
				if !strings.Contains(result.Error, "停止") && !strings.Contains(result.Error, "SIGTERM") {
					t.Errorf("错误消息应该包含停止信息，实际为: %s", result.Error)
				}

				t.Logf("停止测试通过，状态: %s, 错误: %s", result.Status, result.Error)

			case err := <-execErr:
				t.Fatalf("执行命令时出错: %v", err)

			case <-time.After(15 * time.Second):
				t.Fatal("等待执行完成超时")
			}
		})
	}
}

// TestCommandRunner_KillFunctionality 测试强制终止功能
func TestCommandRunner_KillFunctionality(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		killDelay      time.Duration
		expectedStatus core.Status
		description    string
	}{
		{
			name:           "快速强制终止",
			command:        "sleep 10",
			killDelay:      200 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "200ms后强制终止10秒sleep",
		},
		{
			name:           "while循环强制终止",
			command:        "while true; do sleep 1; done",
			killDelay:      300 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "强制终止无限循环命令",
		},
		{
			name:           "忽略信号的命令",
			command:        "trap '' SIGTERM; while true; do sleep 1; done",
			killDelay:      300 * time.Millisecond,
			expectedStatus: core.StatusCanceled,
			description:    "强制终止忽略SIGTERM的命令",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 解析参数
			task := createTestTask(tt.command, "")
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 在goroutine中执行命令
			ctx := context.Background()
			done := make(chan *core.Result, 1)
			execErr := make(chan error, 1)

			go func() {
				logChan := make(chan string, 10)
				defer close(logChan)
				result, err := runner.Execute(ctx, logChan)
				if err != nil {
					execErr <- err
					return
				}
				done <- result
			}()

			// 等待一下确保命令开始执行
			time.Sleep(100 * time.Millisecond)

			// 检查任务是否在运行
			if runner.GetStatus() != core.StatusRunning {
				t.Fatalf("任务应该正在运行，当前状态: %s", runner.GetStatus())
			}

			// 等待指定时间后强制终止
			time.Sleep(tt.killDelay)

			// 强制终止命令
			err = runner.Kill()
			if err != nil {
				t.Fatalf("强制终止命令失败: %v", err)
			}

			// 等待执行完成
			select {
			case result := <-done:
				// 验证结果
				if result.Status != tt.expectedStatus {
					t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
				}

				// 检查错误消息
				if result.Error == "" {
					t.Error("强制终止时错误消息不应该为空")
				}

				// 检查错误消息是否包含终止信息
				if !strings.Contains(result.Error, "强制终止") && !strings.Contains(result.Error, "SIGKILL") {
					t.Errorf("错误消息应该包含强制终止信息，实际为: %s", result.Error)
				}

				t.Logf("强制终止测试通过，状态: %s, 错误: %s", result.Status, result.Error)

			case err := <-execErr:
				t.Fatalf("执行命令时出错: %v", err)

			case <-time.After(15 * time.Second):
				t.Fatal("等待执行完成超时")
			}
		})
	}
}

// TestCommandRunner_ErrorHandling 测试错误处理
func TestCommandRunner_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		expectedStatus core.Status
		expectError    bool
		description    string
	}{
		{
			name:           "不存在的命令",
			command:        "nonexistentcommand12345",
			expectedStatus: core.StatusFailed,
			expectError:    true,
			description:    "执行不存在的命令应该失败",
		},
		{
			name:           "权限不足的命令",
			command:        "cat /root/secret_file_that_does_not_exist",
			expectedStatus: core.StatusFailed,
			expectError:    true,
			description:    "执行权限不足的命令应该失败",
		},
		{
			name:           "语法错误的命令",
			command:        "echo 'unclosed quote",
			expectedStatus: core.StatusFailed,
			expectError:    true,
			description:    "执行语法错误的命令应该失败",
		},
		{
			name:           "返回非零退出码的命令",
			command:        "false",
			expectedStatus: core.StatusFailed,
			expectError:    true,
			description:    "执行返回非零退出码的命令应该失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 解析参数
			task := createTestTask(tt.command, "")
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 执行命令
			ctx := context.Background()
			logChan := make(chan string, 10)
			defer close(logChan)

			result, err := runner.Execute(ctx, logChan)
			if err != nil {
				t.Fatalf("执行命令失败: %v", err)
			}

			// 验证状态
			if result.Status != tt.expectedStatus {
				t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
			}

			// 验证错误处理
			if tt.expectError {
				if result.Error == "" {
					t.Error("期望有错误信息，但没有")
				}
			} else {
				if result.Error != "" {
					t.Errorf("不期望有错误信息，但得到: %s", result.Error)
				}
			}

			t.Logf("错误处理测试通过，状态: %s, 错误: %s", result.Status, result.Error)
		})
	}
}

// TestCommandRunner_ComplexCommands 测试复杂命令
func TestCommandRunner_ComplexCommands(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		expectedStatus core.Status
		expectedOutput string
		description    string
	}{
		{
			name:           "管道命令",
			command:        "echo 'hello world' | wc -w",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "2",
			description:    "测试管道操作",
		},
		{
			name:           "重定向命令",
			command:        "echo 'test output' > /tmp/test_output.txt && cat /tmp/test_output.txt",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "test output",
			description:    "测试重定向操作",
		},
		{
			name:           "逻辑与命令",
			command:        "echo 'first' && echo 'second'",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "first\nsecond",
			description:    "测试逻辑与操作",
		},
		{
			name:           "逻辑或命令",
			command:        "false || echo 'fallback'",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "fallback",
			description:    "测试逻辑或操作",
		},
		{
			name:           "变量替换",
			command:        "echo \"Current user: $USER\"",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "", // 输出会变化，只检查状态
			description:    "测试变量替换",
		},
		{
			name:           "命令替换",
			command:        "echo \"Current date: $(date)\"",
			expectedStatus: core.StatusSuccess,
			expectedOutput: "", // 输出会变化，只检查状态
			description:    "测试命令替换",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()

			// 解析参数
			task := createTestTask(tt.command, "")
			err := runner.ParseArgs(task)
			if err != nil {
				t.Fatalf("解析参数失败: %v", err)
			}

			// 执行命令
			ctx := context.Background()
			logChan := make(chan string, 10)
			defer close(logChan)

			result, err := runner.Execute(ctx, logChan)
			if err != nil {
				t.Fatalf("执行命令失败: %v", err)
			}

			// 验证状态
			if result.Status != tt.expectedStatus {
				t.Errorf("期望状态为 %s，实际为 %s", tt.expectedStatus, result.Status)
			}

			// 验证输出（如果指定了期望输出）
			if tt.expectedOutput != "" {
				actualOutput := strings.TrimSpace(result.Output)
				expectedOutput := strings.TrimSpace(tt.expectedOutput)
				if actualOutput != expectedOutput {
					t.Errorf("期望输出为 %q，实际为 %q", expectedOutput, actualOutput)
				}
			}

			t.Logf("复杂命令测试通过，状态: %s, 输出: %q", result.Status, result.Output)
		})
	}
}

// TestCommandRunner_StatusManagement 测试状态管理
func TestCommandRunner_StatusManagement(t *testing.T) {
	runner := NewCommandRunner()

	// 测试初始状态
	if runner.GetStatus() != core.StatusPending {
		t.Errorf("期望初始状态为 %s，实际为 %s", core.StatusPending, runner.GetStatus())
	}

	// 测试解析参数后的状态
	task := createTestTask("echo", "test")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	if runner.GetStatus() != core.StatusPending {
		t.Errorf("解析参数后状态应该仍为 %s，实际为 %s", core.StatusPending, runner.GetStatus())
	}

	// 测试执行过程中的状态变化
	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		logChan := make(chan string, 10)
		defer close(logChan)
		runner.Execute(ctx, logChan)
		close(done)
	}()

	// 等待一下确保命令开始执行
	time.Sleep(100 * time.Millisecond)

	// 检查运行状态
	status := runner.GetStatus()
	if status != core.StatusRunning && status != core.StatusSuccess {
		t.Errorf("期望状态为 %s 或 %s，实际为 %s", core.StatusRunning, core.StatusSuccess, status)
	}

	// 等待执行完成
	<-done

	// 检查最终状态
	finalStatus := runner.GetStatus()
	if finalStatus != core.StatusSuccess {
		t.Errorf("期望最终状态为 %s，实际为 %s", core.StatusSuccess, finalStatus)
	}
}

// TestCommandRunner_Cleanup 测试资源清理
func TestCommandRunner_Cleanup(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数
	task := createTestTask("echo", "test")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 执行命令
	ctx := context.Background()
	logChan := make(chan string, 10)
	defer close(logChan)

	_, err = runner.Execute(ctx, logChan)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 验证执行后有结果
	if runner.GetResult() == nil {
		t.Fatal("执行后应该有结果")
	}

	// 清理资源
	err = runner.Cleanup()
	if err != nil {
		t.Fatalf("清理资源失败: %v", err)
	}

	// 验证状态已重置
	if runner.GetStatus() != core.StatusPending {
		t.Errorf("期望清理后状态为 %s，实际为 %s", core.StatusPending, runner.GetStatus())
	}

	if runner.GetResult() != nil {
		t.Error("期望清理后结果为nil")
	}
}

// TestCommandRunner_Registry 测试注册表功能
func TestCommandRunner_Registry(t *testing.T) {
	// 测试从注册表创建Runner
	runner, err := core.CreateRunner("command")
	if err != nil {
		t.Fatalf("创建Runner失败: %v", err)
	}

	// 验证类型
	if _, ok := runner.(*CommandRunner); !ok {
		t.Fatal("期望创建的是CommandRunner类型")
	}

	// 测试列出所有Runner类型
	runners := core.ListRunners()
	found := false
	for _, r := range runners {
		if r == "command" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("期望在Runner列表中找到command类型")
	}
}

// TestCommandRunner_ConcurrentExecution 测试并发执行
func TestCommandRunner_ConcurrentExecution(t *testing.T) {
	const numGoroutines = 5
	const numCommands = 3

	results := make(chan *core.Result, numGoroutines*numCommands)
	errors := make(chan error, numGoroutines*numCommands)

	// 启动多个goroutine并发执行命令
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numCommands; j++ {
				runner := NewCommandRunner()

				// 解析参数
				command := fmt.Sprintf("echo 'goroutine %d command %d'", goroutineID, j)
				task := createTestTask(command, "")
				err := runner.ParseArgs(task)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d command %d 解析失败: %v", goroutineID, j, err)
					return
				}

				// 执行命令
				ctx := context.Background()
				logChan := make(chan string, 10)
				defer close(logChan)

				result, err := runner.Execute(ctx, logChan)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d command %d 执行失败: %v", goroutineID, j, err)
					return
				}

				results <- result
			}
		}(i)
	}

	// 收集结果
	successCount := 0
	errorCount := 0
	timeout := time.After(30 * time.Second)

	for i := 0; i < numGoroutines*numCommands; i++ {
		select {
		case result := <-results:
			if result.Status == core.StatusSuccess {
				successCount++
			}
		case err := <-errors:
			t.Logf("并发执行错误: %v", err)
			errorCount++
		case <-timeout:
			t.Fatal("并发执行超时")
		}
	}

	// 验证结果
	if successCount == 0 {
		t.Fatal("没有成功的并发执行")
	}

	t.Logf("并发执行完成: 成功 %d, 错误 %d", successCount, errorCount)
}

// TestCommandRunner_LogChannel 测试日志通道功能
func TestCommandRunner_LogChannel(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数
	task := createTestTask("echo", "test log message")
	err := runner.ParseArgs(task)
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 创建日志通道
	logChan := make(chan string, 10)
	defer close(logChan)

	// 在goroutine中收集日志
	var logs []string
	go func() {
		for log := range logChan {
			if log != "" {
				logs = append(logs, log)
			}
		}
	}()

	// 执行命令
	ctx := context.Background()
	_, err = runner.Execute(ctx, logChan)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 等待日志收集完成
	time.Sleep(100 * time.Millisecond)

	// 验证日志
	if len(logs) == 0 {
		t.Error("期望有日志输出，但没有")
	}

	// 验证日志内容
	found := false
	for _, log := range logs {
		if strings.Contains(log, "test log message") {
			found = true
			break
		}
	}
	if !found {
		t.Error("期望日志包含命令输出")
	}

	t.Logf("日志通道测试通过，收集到 %d 条日志", len(logs))
}

// TestCommandRunner_EdgeCases 测试边界情况
func TestCommandRunner_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        string
		expectError bool
		description string
	}{
		{
			name:        "空字符串命令",
			command:     "",
			args:        "",
			expectError: true,
			description: "空字符串命令应该返回错误",
		},
		{
			name:        "只有空格的命令",
			command:     "   ",
			args:        "",
			expectError: true,
			description: "只有空格的命令应该返回错误",
		},
		{
			name:        "只有空格的args",
			command:     "echo",
			args:        "   ",
			expectError: false,
			description: "只有空格的args应该被处理",
		},
		{
			name:        "特殊字符命令",
			command:     "echo 'hello; world'",
			args:        "",
			expectError: false,
			description: "包含特殊字符的命令应该被处理",
		},
		{
			name:        "长命令",
			command:     "echo " + strings.Repeat("a", 1000),
			args:        "",
			expectError: false,
			description: "长命令应该被处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewCommandRunner()
			task := createTestTask(tt.command, tt.args)
			err := runner.ParseArgs(task)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望返回错误，但没有错误")
				}
			} else {
				if err != nil {
					t.Errorf("不期望返回错误，但得到: %v", err)
				}
			}
		})
	}
}

// BenchmarkCommandRunner_Execute 性能基准测试
func BenchmarkCommandRunner_Execute(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		runner := NewCommandRunner()
		task := createTestTask("echo", "benchmark test")
		err := runner.ParseArgs(task)
		if err != nil {
			b.Fatalf("解析参数失败: %v", err)
		}

		ctx := context.Background()
		logChan := make(chan string, 10)

		result, err := runner.Execute(ctx, logChan)
		if err != nil {
			b.Fatalf("执行命令失败: %v", err)
		}

		if result.Status != core.StatusSuccess {
			b.Fatalf("期望状态为success，实际为: %s", result.Status)
		}

		close(logChan)
	}
}
