package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CommandRunner 命令执行器
//
// 用于执行系统命令和脚本的Runner实现
// 使用 bash -c 执行完整的命令字符串，支持复杂的shell命令
// 包括管道、重定向、逻辑操作符等
type CommandRunner struct {
	task           *core.Task    // 完整的任务对象，包含所有配置信息
	command        string        // 最终执行的完整命令字符串（从task.command和task.args解析组合）
	timeout        time.Duration // 执行超时时间（从task中提取，便于理解和操作）
	status         core.Status   // 当前状态
	result         *core.Result  // 执行结果
	cmd            *exec.Cmd     // 执行命令对象
	mutex          sync.RWMutex  // 读写锁
	stopSignalType string        // 🔥 用户停止信号类型（""=未停止, "SIGTERM"=优雅停止, "SIGKILL"=强制终止）
}

// NewCommandRunner 创建新的CommandRunner实例
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{
		status: core.StatusPending,
	}
}

// ParseArgs 解析任务参数和配置
func (r *CommandRunner) ParseArgs(task *core.Task) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 保存任务对象
	r.task = task
	if r.task == nil {
		return fmt.Errorf("任务对象未设置")
	} else if r.task.ID == uuid.Nil {
		return fmt.Errorf("任务ID未设置")
	}

	// 🔥 提取超时时间，增加安全默认值
	if task.Timeout > 0 {
		// 使用用户指定的超时时间
		r.timeout = time.Duration(task.Timeout) * time.Second
	} else {
		// 🔥 Timeout=0时，使用安全默认值（24小时）
		// 原因：
		// 1. 防止任务无限期运行，耗尽系统资源
		// 2. 24小时对于绝大多数CronJob任务已经足够
		// 3. 如果用户确实需要更长时间，应该显式设置Timeout
		r.timeout = 24 * time.Hour

		logger.Debug("任务未设置执行超时时间，使用安全默认值",
			zap.String("task_id", task.ID.String()),
			zap.Duration("default_timeout", r.timeout))
	}

	// 专业地解析和构建完整的命令字符串
	fullCommand, err := r.buildFullCommand(task.Command, task.Args)
	if err != nil {
		return fmt.Errorf("构建命令失败: %w", err)
	}
	r.command = fullCommand

	// 检查命令是否为空
	if strings.TrimSpace(r.command) == "" {
		return fmt.Errorf("命令不能为空")
	}

	// 执行安全检查
	baseCommand := r.extractBaseCommand(r.command)
	if err := GetGlobalSecurity().ValidateCommand(baseCommand, []string{}); err != nil {
		return fmt.Errorf("安全检查失败: %w", err)
	}

	return nil
}

// buildFullCommand 专业地构建完整的命令字符串
// 正确处理JSON数组格式的args参数
func (r *CommandRunner) buildFullCommand(command, args string) (string, error) {
	// 如果args为空或空数组，直接返回command
	if args == "" || args == "[]" || strings.TrimSpace(args) == "" {
		return command, nil
	}

	// 尝试解析args为JSON数组
	var argsList []string
	if err := json.Unmarshal([]byte(args), &argsList); err != nil {
		// 如果JSON解析失败，可能args是普通字符串，直接拼接
		// 但这种情况需要谨慎处理，因为可能存在注入风险
		return command + " " + args, nil
	}

	// 如果是空数组，直接返回command
	if len(argsList) == 0 {
		return command, nil
	}

	// 将参数数组安全地拼接到命令后面
	// 每个参数都需要适当的转义和引用
	var fullCommand strings.Builder
	fullCommand.WriteString(command)

	for _, arg := range argsList {
		fullCommand.WriteString(" ")
		// 如果参数包含空格或特殊字符，需要加引号
		if r.needsQuoting(arg) {
			fullCommand.WriteString("'")
			// 转义单引号
			escapedArg := strings.ReplaceAll(arg, "'", "'\"'\"'")
			fullCommand.WriteString(escapedArg)
			fullCommand.WriteString("'")
		} else {
			fullCommand.WriteString(arg)
		}
	}

	return fullCommand.String(), nil
}

// needsQuoting 检查参数是否需要加引号
func (r *CommandRunner) needsQuoting(arg string) bool {
	// 如果包含空格、特殊字符等，需要加引号
	specialChars := " \t\n\r\"'\\|&;<>()$`*?[]{}"
	for _, char := range specialChars {
		if strings.ContainsRune(arg, char) {
			return true
		}
	}
	return false
}

// extractBaseCommand 从完整命令字符串中提取基础命令
// 例如: "ls -la" -> "ls", "bash -c 'echo hello'" -> "bash"
func (r *CommandRunner) extractBaseCommand(fullCommand string) string {
	// 去除首尾空格
	command := strings.TrimSpace(fullCommand)

	// 如果命令为空，返回空字符串
	if command == "" {
		return ""
	}

	// 按空格分割，取第一个部分作为基础命令
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	// 返回第一个部分作为基础命令
	return parts[0]
}

// updateStatus 更新状态和结果
func (r *CommandRunner) updateStatus(status core.Status, errorMsg string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.status = status
	if r.result != nil {
		r.result.Status = status
		r.result.Error = errorMsg
	} else {
		r.result = &core.Result{
			Status:    status,
			Error:     errorMsg,
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  0,
		}
	}
}

// Execute 执行任务
func (r *CommandRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	// 检查状态
	r.mutex.Lock()
	if r.status != core.StatusPending {
		r.mutex.Unlock()
		return nil, fmt.Errorf("任务状态不正确，当前状态: %s", r.status)
	}

	if r.task == nil {
		r.mutex.Unlock()
		return nil, fmt.Errorf("任务对象未设置，请先调用ParseArgs")
	}

	if r.command == "" {
		r.mutex.Unlock()
		return nil, fmt.Errorf("命令未设置，请先调用ParseArgs")
	}

	// 设置状态为运行中
	r.status = core.StatusRunning
	startTime := time.Now()

	// 创建带超时的上下文
	var execCtx context.Context
	var cancel context.CancelFunc
	if r.timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, r.timeout)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	r.mutex.Unlock()

	// 创建执行命令 - 使用 bash -c 执行完整命令字符串
	r.cmd = exec.CommandContext(execCtx, "bash", "-c", r.command)

	// 🔥 设置进程组，确保信号能传递到所有子进程
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // 创建新的进程组
	}

	// 设置工作目录
	workingDir, err := r.setupWorkingDirectory()
	if err != nil {
		r.mutex.Lock()
		r.updateStatus(core.StatusError, fmt.Sprintf("工作目录设置失败: %v", err))
		r.mutex.Unlock()
		return r.result, err
	}
	r.cmd.Dir = workingDir

	// 设置环境变量
	if r.task != nil && len(r.task.Metadata) > 0 {
		if metadata, err := r.task.GetMetadata(); err == nil && len(metadata.Environment) > 0 {
			// 继承系统环境变量
			r.cmd.Env = append(r.cmd.Env, os.Environ()...)
			// 添加任务特定的环境变量
			for key, value := range metadata.Environment {
				r.cmd.Env = append(r.cmd.Env, fmt.Sprintf("%s=%s", key, value))
			}
		}
	}

	// 设置输出捕获
	var stdout, stderr bytes.Buffer
	r.cmd.Stdout = &stdout
	r.cmd.Stderr = &stderr

	// 执行命令
	err = r.cmd.Run()

	// 设置结束时间
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	// 获取输出内容
	output := stdout.String()
	errorOutput := stderr.String()

	// 构建结果
	r.mutex.Lock()
	r.result = &core.Result{
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		Output:     output, // 用于后续任务取数据
		ExecuteLog: output, // 用于显示给用户的执行日志（与Output相同）
	}

	// 处理执行结果
	if err != nil {
		// 🔥 优先检查是否被用户停止（避免被识别为error触发重试）
		if r.stopSignalType != "" {
			r.status = core.StatusStopped
			r.result.Status = core.StatusStopped
			r.result.Error = fmt.Sprintf("任务被用户停止 (发送%s信号)\n", r.stopSignalType)
		} else if err == context.DeadlineExceeded {
			// 在锁内处理错误状态
			r.status = core.StatusTimeout
			r.result.Status = core.StatusTimeout
			r.result.Error = fmt.Sprintf("任务执行超时 (超时时间: %v)\n", r.timeout)
		} else if err == context.Canceled {
			r.status = core.StatusCanceled
			r.result.Status = core.StatusCanceled
			r.result.Error = "任务被取消"
		} else {
			// 检查是否是信号杀死
			if strings.Contains(err.Error(), "signal: killed") {
				// 检查是否是超时导致的信号杀死
				if r.timeout > 0 {
					// 超时导致的信号杀死
					r.status = core.StatusTimeout
					r.result.Status = core.StatusTimeout
					r.result.Error = fmt.Sprintf("任务执行超时 (超时时间: %v)\n", r.timeout)
				} else {
					// 其他信号杀死（包括SIGKILL）
					r.status = core.StatusCanceled
					r.result.Status = core.StatusCanceled
					r.result.Error = "任务被强制终止 (SIGKILL信号)\n"
				}
			} else {
				r.status = core.StatusFailed
				r.result.Status = core.StatusFailed
				r.result.Error = err.Error()
			}
		}
		// 把错误消息给发送出去
		if logChan != nil && r.result.Error != "" {
			logChan <- r.result.Error
		}
	} else {
		// 任务正常完成
		r.status = core.StatusSuccess
		r.result.Status = core.StatusSuccess

		// 🔥 如果用户尝试停止但任务已经完成，在日志中说明
		if r.stopSignalType != "" {
			note := fmt.Sprintf("\n[注意] 用户尝试发送%s信号停止任务，但任务已正常完成", r.stopSignalType)
			r.result.ExecuteLog += note
			if logChan != nil {
				logChan <- note
			}
		}
	}

	r.mutex.Unlock()

	// 设置退出码
	if r.cmd.ProcessState != nil {
		r.result.ExitCode = r.cmd.ProcessState.ExitCode()
	}

	// 如果有stderr输出，添加到错误信息中
	if errorOutput != "" {
		if r.result.Error != "" {
			r.result.Error += "\n" + errorOutput
		} else {
			r.result.Error = errorOutput
		}
		// 同时添加到ExecuteLog中
		r.result.ExecuteLog += "\n" + errorOutput
	}

	result := r.result

	// 如果有日志通道，发送执行日志
	if logChan != nil && result.ExecuteLog != "" {
		select {
		case logChan <- result.ExecuteLog:
		default:
			// 如果通道已满，跳过发送
		}
		// 写入个空，就是结束了
		logChan <- ""

		// 等待接收完成，确保发送顺序
		time.Sleep(50 * time.Millisecond)
	}

	return result, nil
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

		// 🔥 记录用户停止信号类型（确保不会被识别为error触发重试）
		r.stopSignalType = "SIGTERM"

		// 🔥 发送SIGTERM信号到进程组（确保子进程也能收到信号）
		// 注意：不要调用cmd.Wait()，因为Execute()中的cmd.Run()会处理它
		// cmd.Wait()只能被调用一次，重复调用会导致死锁
		pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
		if err == nil {
			// 向进程组发送SIGTERM信号
			if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				// 如果向进程组发送失败，尝试向单个进程发送
				if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
					return fmt.Errorf("发送SIGTERM信号失败: %w", err)
				}
			}
		} else {
			// 如果获取进程组失败，直接向进程发送信号
			if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("发送SIGTERM信号失败: %w", err)
			}
		}

		logger.Info("已发送SIGTERM信号",
			zap.String("task_id", r.task.ID.String()),
			zap.Int("pid", r.cmd.Process.Pid))
	}

	return nil
}

// Kill 强制终止任务执行
func (r *CommandRunner) Kill() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		// 🔥 记录用户停止信号类型（确保不会被识别为error触发重试）
		r.stopSignalType = "SIGKILL"

		// 🔥 发送SIGKILL信号到进程组（确保子进程也能被终止）
		pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
		if err == nil {
			// 向进程组发送SIGKILL信号
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
				// 如果向进程组发送失败，尝试向单个进程发送
				if err := r.cmd.Process.Signal(syscall.SIGKILL); err != nil {
					return fmt.Errorf("发送SIGKILL信号失败: %w", err)
				}
			}
		} else {
			// 如果获取进程组失败，直接向进程发送信号
			if err := r.cmd.Process.Signal(syscall.SIGKILL); err != nil {
				return fmt.Errorf("发送SIGKILL信号失败: %w", err)
			}
		}

		logger.Info("已发送SIGKILL信号",
			zap.String("task_id", r.task.ID.String()),
			zap.Int("pid", r.cmd.Process.Pid))
	}

	return nil
}

// GetStatus 获取当前执行状态
func (r *CommandRunner) GetStatus() core.Status {
	return r.status
}

// GetResult 获取执行结果
func (r *CommandRunner) GetResult() *core.Result {
	return r.result
}

// setupWorkingDirectory 设置并确保工作目录存在
//
// 根据元数据配置或任务ID生成工作目录路径，并确保目录存在
// 返回最终使用的工作目录路径和可能的错误
func (r *CommandRunner) setupWorkingDirectory() (string, error) {
	var workingDir string

	// 如果任务有CronJob，则使用CronJob的名称作为工作目录
	if r.task.CronJob != nil {
		workingDir = filepath.Join(config.WorkerInstance.WorkingDir, "tasks", r.task.CronJob.String())
	} else {
		workingDir = filepath.Join(config.WorkerInstance.WorkingDir, "tasks", r.task.ID.String())
	}

	// 实时从task获取元数据: 如果任务有元数据，则使用元数据中配置的工作目录
	if r.task != nil && len(r.task.Metadata) > 0 {
		if metadata, err := r.task.GetMetadata(); err == nil && metadata.WorkingDir != "" {
			// 使用元数据中配置的工作目录
			workingDir = metadata.WorkingDir
		}
	}

	// 如果不是当前目录，检查并创建目录
	if workingDir != "." {
		// 转换为绝对路径以便更好地处理
		absPath, err := filepath.Abs(workingDir)
		if err != nil {
			return "", fmt.Errorf("无法解析工作目录路径 %s: %w", workingDir, err)
		}

		// 检查目录是否存在
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			// 目录不存在，创建目录
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return "", fmt.Errorf("无法创建工作目录 %s: %w", absPath, err)
			}
		} else if err != nil {
			// 其他错误（权限问题等）
			return "", fmt.Errorf("无法访问工作目录 %s: %w", absPath, err)
		}

		// 检查是否为目录
		if stat, err := os.Stat(absPath); err == nil {
			if !stat.IsDir() {
				return "", fmt.Errorf("工作目录路径 %s 不是一个目录", absPath)
			}
		}

		workingDir = absPath
	}

	return workingDir, nil
}

// Cleanup 清理资源
func (r *CommandRunner) Cleanup() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

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

	return nil
}

// 确保CommandRunner实现了Runner接口
var _ core.Runner = (*CommandRunner)(nil)
