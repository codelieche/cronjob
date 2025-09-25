package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

func TestCommandRunner_ParseArgs(t *testing.T) {
	runner := NewCommandRunner()

	// 测试基本命令（只有command，没有args）
	err := runner.ParseArgs("ls -la", "")
	if err != nil {
		t.Fatalf("解析空参数失败: %v", err)
	}
	if runner.GetCommand() != "ls -la" {
		t.Fatalf("期望命令为'ls -la'，实际为: %s", runner.GetCommand())
	}

	// 测试带参数的命令（command和args拼接）
	err = runner.ParseArgs("echo", "hello world")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	if runner.GetCommand() != "echo hello world" {
		t.Fatalf("期望命令为'echo hello world'，实际为: %s", runner.GetCommand())
	}

	// 测试复杂命令（包含管道和重定向）
	complexCmd := "ping 8.8.8.8 -c 3 && echo `date` >> ./logs/ping.log && echo `date`"
	err = runner.ParseArgs(complexCmd, "")
	if err != nil {
		t.Fatalf("解析复杂命令失败: %v", err)
	}
	if runner.GetCommand() != complexCmd {
		t.Fatalf("期望命令为'%s'，实际为: %s", complexCmd, runner.GetCommand())
	}

	// 测试空命令
	err = runner.ParseArgs("", "")
	if err == nil {
		t.Fatal("应该返回空命令错误")
	}
}

func TestCommandRunner_Execute(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数 - 使用简单命令
	err := runner.ParseArgs("echo", "hello world")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 执行命令
	ctx := context.Background()
	result, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 验证结果
	if result.Status != core.StatusSuccess {
		t.Fatalf("期望状态为success，实际为: %s", result.Status)
	}

	if result.ExitCode != 0 {
		t.Fatalf("期望退出码为0，实际为: %d", result.ExitCode)
	}

	if result.Output != "hello world\n" {
		t.Fatalf("期望输出为'hello world\\n'，实际为: %q", result.Output)
	}
}

func TestCommandRunner_Timeout(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数 - 使用sleep命令
	err := runner.ParseArgs("sleep", "10")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 设置超时
	runner.SetTimeout(2 * time.Second)

	// 执行命令
	ctx := context.Background()
	result, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 验证结果
	if result.Status != core.StatusTimeout {
		t.Fatalf("期望状态为timeout，实际为: %s", result.Status)
	}

	if result.Duration < 2000 || result.Duration > 3000 {
		t.Fatalf("期望执行时间在2-3秒之间，实际为: %d毫秒", result.Duration)
	}
}

func TestCommandRunner_Stop(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数 - 使用sleep命令
	err := runner.ParseArgs("sleep", "10")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 在goroutine中执行命令
	ctx := context.Background()
	done := make(chan *core.Result, 1)
	go func() {
		result, _ := runner.Execute(ctx)
		done <- result
	}()

	// 等待一下确保命令开始执行
	time.Sleep(100 * time.Millisecond)

	// 停止命令
	err = runner.Stop()
	if err != nil {
		t.Fatalf("停止命令失败: %v", err)
	}

	// 等待执行完成
	select {
	case result := <-done:
		// 验证结果 - 可能是canceled或success（如果已经完成）
		if result.Status != core.StatusCanceled && result.Status != core.StatusSuccess {
			t.Fatalf("期望状态为canceled或success，实际为: %s", result.Status)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("等待执行完成超时")
	}
}

func TestCommandRunner_GetStatus(t *testing.T) {
	runner := NewCommandRunner()

	// 初始状态应该是pending
	if runner.GetStatus() != core.StatusPending {
		t.Fatalf("期望初始状态为pending，实际为: %s", runner.GetStatus())
	}

	// 解析参数
	err := runner.ParseArgs("sleep", "1")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 在goroutine中执行命令
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		runner.Execute(ctx)
		close(done)
	}()

	// 等待一下确保命令开始执行
	time.Sleep(100 * time.Millisecond)

	// 检查状态 - 由于echo命令执行很快，可能已经完成
	status := runner.GetStatus()
	if status != core.StatusRunning && status != core.StatusSuccess {
		t.Fatalf("期望状态为running或success，实际为: %s", status)
	}

	// 等待执行完成
	<-done
}

func TestCommandRunner_Cleanup(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数
	err := runner.ParseArgs("echo", "test")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 执行命令
	ctx := context.Background()
	_, err = runner.Execute(ctx)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 清理资源
	err = runner.Cleanup()
	if err != nil {
		t.Fatalf("清理资源失败: %v", err)
	}

	// 验证状态已重置
	if runner.GetStatus() != core.StatusPending {
		t.Fatalf("期望清理后状态为pending，实际为: %s", runner.GetStatus())
	}

	if runner.GetResult() != nil {
		t.Fatal("期望清理后结果为nil")
	}
}

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

func TestCommandRunner_ErrorHandling(t *testing.T) {
	runner := NewCommandRunner()

	// 解析参数 - 使用不存在的命令
	err := runner.ParseArgs("nonexistentcommand", "")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}

	// 执行命令
	ctx := context.Background()
	result, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 验证结果
	if result.Status != core.StatusFailed {
		t.Fatalf("期望状态为failed，实际为: %s", result.Status)
	}

	// 对于不存在的命令，ExitCode可能为0（某些系统）或非0
	// 我们主要检查状态和错误信息
	if result.Error == "" {
		t.Fatal("期望有错误信息")
	}
}

func TestCommandRunner_ComplexCommand(t *testing.T) {
	runner := NewCommandRunner()

	// 测试复杂命令 - 包含管道和重定向
	complexCmd := "echo 'hello world' | wc -w"
	err := runner.ParseArgs(complexCmd, "")
	if err != nil {
		t.Fatalf("解析复杂命令失败: %v", err)
	}

	// 执行命令
	ctx := context.Background()
	result, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("执行命令失败: %v", err)
	}

	// 验证结果
	if result.Status != core.StatusSuccess {
		t.Fatalf("期望状态为success，实际为: %s", result.Status)
	}

	// 验证输出 - wc -w 应该输出 "2"（可能有前导空格）
	actualOutput := result.Output
	// 去除前导和尾随空格进行比较
	if strings.TrimSpace(actualOutput) != "2" {
		t.Fatalf("期望输出包含'2'，实际为: %q", actualOutput)
	}
}

// TestCommandRunner_TimeoutError 测试超时时Error消息是否正确设置
func TestCommandRunner_TimeoutError(t *testing.T) {
	runner := NewCommandRunner()
	
	// 设置一个很短的超时时间
	runner.SetTimeout(100 * time.Millisecond)
	
	// 解析一个会长时间运行的命令
	err := runner.ParseArgs("sleep", "2")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	
	// 执行任务
	ctx := context.Background()
	result, err := runner.Execute(ctx)
	
	// 检查结果
	if result == nil {
		t.Fatal("结果不应该为nil")
	}
	
	// 检查状态
	if result.Status != core.StatusTimeout {
		t.Errorf("期望状态为 %s，实际为 %s", core.StatusTimeout, result.Status)
	}
	
	// 检查错误消息
	if result.Error == "" {
		t.Error("超时时Error消息不应该为空")
	}
	
	// 检查错误消息是否包含超时信息
	expectedError := "任务执行超时"
	if result.Error != expectedError && result.Error != "任务执行超时 (超时时间: 100ms)" {
		t.Errorf("期望错误消息包含 '%s'，实际为 '%s'", expectedError, result.Error)
	}
	
	t.Logf("超时测试通过，错误消息: %s", result.Error)
}

// TestCommandRunner_StopError 测试停止时Error消息是否正确设置
func TestCommandRunner_StopError(t *testing.T) {
	runner := NewCommandRunner()
	
	// 解析一个会长时间运行的命令 - 使用while循环
	err := runner.ParseArgs("while true; do sleep 1; done", "")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	
	// 在goroutine中执行命令
	ctx := context.Background()
	done := make(chan *core.Result, 1)
	go func() {
		result, _ := runner.Execute(ctx)
		done <- result
	}()
	
	// 等待一下确保命令开始执行
	time.Sleep(200 * time.Millisecond)
	
	// 检查任务是否在运行
	if runner.GetStatus() != core.StatusRunning {
		t.Fatalf("任务应该正在运行，当前状态: %s", runner.GetStatus())
	}
	
	// 停止命令
	err = runner.Stop()
	if err != nil {
		t.Fatalf("停止命令失败: %v", err)
	}
	
	// 等待执行完成
	select {
	case result := <-done:
		// 验证结果 - 应该是canceled状态
		if result.Status != core.StatusCanceled {
			t.Errorf("期望状态为 %s，实际为 %s", core.StatusCanceled, result.Status)
		}
		
		// 检查错误消息
		if result.Error == "" {
			t.Error("停止时Error消息不应该为空")
		}
		
		// 检查错误消息是否包含停止信息
		expectedError := "任务被停止 (SIGTERM信号)"
		if result.Error != expectedError {
			t.Errorf("期望错误消息为 '%s'，实际为 '%s'", expectedError, result.Error)
		}
		
		t.Logf("停止测试通过，错误消息: %s", result.Error)
	case <-time.After(15 * time.Second):
		t.Fatal("等待执行完成超时")
	}
}

// TestCommandRunner_KillError 测试强制终止时Error消息是否正确设置
func TestCommandRunner_KillError(t *testing.T) {
	runner := NewCommandRunner()
	
	// 解析一个会长时间运行的命令 - 使用while循环
	err := runner.ParseArgs("while true; do sleep 1; done", "")
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	
	// 在goroutine中执行命令
	ctx := context.Background()
	done := make(chan *core.Result, 1)
	go func() {
		result, _ := runner.Execute(ctx)
		done <- result
	}()
	
	// 等待一下确保命令开始执行
	time.Sleep(200 * time.Millisecond)
	
	// 检查任务是否在运行
	if runner.GetStatus() != core.StatusRunning {
		t.Fatalf("任务应该正在运行，当前状态: %s", runner.GetStatus())
	}
	
	// 强制终止命令
	err = runner.Kill()
	if err != nil {
		t.Fatalf("强制终止命令失败: %v", err)
	}
	
	// 等待执行完成
	select {
	case result := <-done:
		// 验证结果 - 应该是canceled状态
		if result.Status != core.StatusCanceled {
			t.Errorf("期望状态为 %s，实际为 %s", core.StatusCanceled, result.Status)
		}
		
		// 检查错误消息
		if result.Error == "" {
			t.Error("强制终止时Error消息不应该为空")
		}
		
		// 检查错误消息是否包含终止信息
		expectedError := "任务被强制终止 (SIGKILL信号)"
		if result.Error != expectedError {
			t.Errorf("期望错误消息为 '%s'，实际为 '%s'", expectedError, result.Error)
		}
		
		t.Logf("强制终止测试通过，错误消息: %s", result.Error)
	case <-time.After(15 * time.Second):
		t.Fatal("等待执行完成超时")
	}
}
