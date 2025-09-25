package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// CommandRunner 命令执行器
//
// 用于执行系统命令和脚本的Runner实现
// 使用 bash -c 执行完整的命令字符串，支持复杂的shell命令
// 包括管道、重定向、逻辑操作符等
type CommandRunner struct {
	command   string             // 完整的命令字符串
	timeout   time.Duration      // 执行超时时间
	status    core.Status        // 当前状态
	result    *core.Result       // 执行结果
	cmd       *exec.Cmd          // 执行命令对象
	startTime time.Time          // 开始时间
	endTime   time.Time          // 结束时间
	mutex     sync.RWMutex       // 读写锁
	ctx       context.Context    // 执行上下文
	cancel    context.CancelFunc // 取消函数
}

// NewCommandRunner 创建新的CommandRunner实例
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		status: core.StatusPending,
	}
}

// ParseArgs 解析任务参数
func (r *CommandRunner) ParseArgs(command string, args string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 构建完整的命令字符串
	if args == "" || args == "[]" {
		// 如果args为空，直接使用command作为完整命令
		r.command = command
	} else {
		// 如果args不为空，将command和args拼接为完整命令
		// 这里简单地将args作为字符串拼接，不进行JSON解析
		r.command = command + " " + args
	}

	// 检查命令是否为空
	if strings.TrimSpace(r.command) == "" {
		return fmt.Errorf("命令不能为空")
	}

	// 执行安全检查 - 这里需要调整安全检查逻辑
	// 暂时注释掉，因为现在command是完整字符串而不是分离的command和args
	// if err := GetGlobalSecurity().ValidateCommand(r.command, []string{}); err != nil {
	// 	return fmt.Errorf("安全检查失败: %w", err)
	// }

	return nil
}


// Execute 执行任务
func (r *CommandRunner) Execute(ctx context.Context) (*core.Result, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 检查状态
	if r.status != core.StatusPending {
		return nil, fmt.Errorf("任务状态不正确，当前状态: %s", r.status)
	}

	// 设置状态为运行中
	r.status = core.StatusRunning
	r.startTime = time.Now()

	// 创建带超时的上下文
	if r.timeout > 0 {
		r.ctx, r.cancel = context.WithTimeout(ctx, r.timeout)
	} else {
		r.ctx, r.cancel = context.WithCancel(ctx)
	}
	defer r.cancel()

	// 创建执行命令 - 使用 bash -c 执行完整命令字符串
	r.cmd = exec.CommandContext(r.ctx, "bash", "-c", r.command)

	// 设置输出捕获
	var stdout, stderr bytes.Buffer
	r.cmd.Stdout = &stdout
	r.cmd.Stderr = &stderr

	// 执行命令
	err := r.cmd.Run()

	// 设置结束时间
	r.endTime = time.Now()
	duration := r.endTime.Sub(r.startTime).Milliseconds()

	// 构建结果
	r.result = &core.Result{
		StartTime: r.startTime,
		EndTime:   r.endTime,
		Duration:  duration,
		Output:    stdout.String(),
	}

	// 处理执行结果
	if err != nil {
		// 检查是否是超时错误
		if r.ctx.Err() == context.DeadlineExceeded {
			r.status = core.StatusTimeout
			r.result.Status = core.StatusTimeout
			r.result.Error = fmt.Sprintf("任务执行超时 (超时时间: %v)", r.timeout)
		} else if r.ctx.Err() == context.Canceled {
			r.status = core.StatusCanceled
			r.result.Status = core.StatusCanceled
			r.result.Error = "任务被取消"
		} else {
			r.status = core.StatusFailed
			r.result.Status = core.StatusFailed
			r.result.Error = err.Error()
		}
	} else {
		r.status = core.StatusSuccess
		r.result.Status = core.StatusSuccess
	}

	// 设置退出码
	if r.cmd.ProcessState != nil {
		r.result.ExitCode = r.cmd.ProcessState.ExitCode()
	}

	// 如果有stderr输出，添加到错误信息中
	if stderr.Len() > 0 {
		if r.result.Error != "" {
			r.result.Error += "\n" + stderr.String()
		} else {
			r.result.Error = stderr.String()
		}
	}

	return r.result, nil
}

// Stop 停止任务执行
func (r *CommandRunner) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		// 检查进程是否还在运行
		if r.cmd.ProcessState != nil && r.cmd.ProcessState.Exited() {
			// 进程已经退出，不需要发送信号
			return nil
		}

		// 发送SIGTERM信号
		if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("发送SIGTERM信号失败: %w", err)
		}

		// 等待进程退出
		done := make(chan error, 1)
		go func() {
			done <- r.cmd.Wait()
		}()

		// 等待最多5秒
		select {
		case <-done:
			// 进程已退出
		case <-time.After(5 * time.Second):
			// 超时，强制终止
			return r.Kill()
		}

		// 更新状态
		if r.status == core.StatusRunning {
			r.status = core.StatusCanceled
			if r.result != nil {
				r.result.Status = core.StatusCanceled
				r.result.Error = "任务被停止 (SIGTERM信号)"
			} else {
				// 如果result还没有创建，创建一个新的
				r.result = &core.Result{
					Status:    core.StatusCanceled,
					Error:     "任务被停止 (SIGTERM信号)",
					StartTime: r.startTime,
					EndTime:   time.Now(),
					Duration:  time.Since(r.startTime).Milliseconds(),
				}
			}
		}
	}

	return nil
}

// Kill 强制终止任务执行
func (r *CommandRunner) Kill() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		// 发送SIGKILL信号
		if err := r.cmd.Process.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("发送SIGKILL信号失败: %w", err)
		}

		// 更新状态
		if r.status == core.StatusRunning {
			r.status = core.StatusCanceled
			if r.result != nil {
				r.result.Status = core.StatusCanceled
				r.result.Error = "任务被强制终止 (SIGKILL信号)"
			} else {
				// 如果result还没有创建，创建一个新的
				r.result = &core.Result{
					Status:    core.StatusCanceled,
					Error:     "任务被强制终止 (SIGKILL信号)",
					StartTime: r.startTime,
					EndTime:   time.Now(),
					Duration:  time.Since(r.startTime).Milliseconds(),
				}
			}
		}
	}

	return nil
}

// GetStatus 获取当前执行状态
func (r *CommandRunner) GetStatus() core.Status {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.status
}

// GetResult 获取执行结果
func (r *CommandRunner) GetResult() *core.Result {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.result
}

// SetTimeout 设置执行超时时间
func (r *CommandRunner) SetTimeout(timeout time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.timeout = timeout
}

// GetCommand 获取完整命令字符串（用于测试和调试）
func (r *CommandRunner) GetCommand() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.command
}

// Cleanup 清理资源
func (r *CommandRunner) Cleanup() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 取消上下文
	if r.cancel != nil {
		r.cancel()
	}

	// 清理命令对象
	if r.cmd != nil {
		if r.cmd.Process != nil && r.cmd.ProcessState == nil {
			// 如果进程还在运行，强制终止
			r.cmd.Process.Kill()
		}
		r.cmd = nil
	}

	// 重置状态
	r.status = core.StatusPending
	r.result = nil
	r.startTime = time.Time{}
	r.endTime = time.Time{}
	r.ctx = nil
	r.cancel = nil

	return nil
}

// 确保CommandRunner实现了Runner接口
var _ core.Runner = (*CommandRunner)(nil)
