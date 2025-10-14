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

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ScriptConfig 脚本配置（v1.0 - 标准版）
//
// 支持文件模式和内联模式，但不支持依赖管理（保持简单）
type ScriptConfig struct {
	Language    string   `json:"language"`    // 必填：python/nodejs/shell
	Type        string   `json:"type"`        // 必填：file（文件）或 inline（内联）
	File        string   `json:"file"`        // type=file时必填：脚本文件绝对路径
	Code        string   `json:"code"`        // type=inline时必填：脚本内容
	Args        []string `json:"args"`        // 选填：脚本参数
	Interpreter string   `json:"interpreter"` // 选填：自定义解释器路径，留空则使用默认
}

// ScriptRunner 脚本执行器（v1.0 - 标准版）
//
// 用于执行 Python、Node.js、Shell 等脚本语言
//
// 核心特性：
//   - 支持文件模式：执行已存在的脚本文件
//   - 支持内联模式：将代码保存为临时文件执行
//   - 支持参数传递：通过命令行参数传递
//   - 支持环境变量：从 metadata 注入
//   - 支持工作目录：从 metadata 设置
//   - 不支持依赖管理（v1.0），可在 setup 中处理
//
// 安全措施：
//   - 文件路径白名单验证（配置中设置）
//   - 内联代码长度限制（10KB）
//   - 临时文件自动清理
//   - 超时控制（使用 Task 的 Timeout）
type ScriptRunner struct {
	task           *core.Task    // 完整的任务对象
	config         *ScriptConfig // 脚本配置
	timeout        time.Duration // 执行超时时间
	status         core.Status   // 当前状态
	result         *core.Result  // 执行结果
	cmd            *exec.Cmd     // 执行命令对象
	mutex          sync.RWMutex  // 读写锁
	stopSignalType string        // 停止信号类型（""=未停止, "SIGTERM"=优雅停止, "SIGKILL"=强制终止）
	tempFile       string        // 临时文件路径（内联模式使用）
}

// 脚本文件白名单目录（安全措施）
// 可通过环境变量 ALLOWED_SCRIPT_DIRS 配置，用分号分隔
// 示例：ALLOWED_SCRIPT_DIRS="/var/scripts;/opt/cronjob/scripts"
var allowedScriptDirs = []string{
	"/var/scripts",
	"/opt/cronjob/scripts",
	"/data/scripts",
}

// 内联代码最大长度（10KB）
const maxInlineCodeSize = 10 * 1024

// NewScriptRunner 创建新的ScriptRunner实例
func NewScriptRunner() *ScriptRunner {
	return &ScriptRunner{
		status: core.StatusPending,
	}
}

// ParseArgs 解析任务参数和配置
func (r *ScriptRunner) ParseArgs(task *core.Task) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 保存任务对象
	r.task = task
	if r.task == nil {
		return fmt.Errorf("任务对象未设置")
	} else if r.task.ID == uuid.Nil {
		return fmt.Errorf("任务ID未设置")
	}

	// 提取超时时间
	if task.Timeout > 0 {
		r.timeout = time.Duration(task.Timeout) * time.Second
	} else {
		// 默认24小时超时（安全默认值）
		r.timeout = 24 * time.Hour
		logger.Debug("任务未设置超时时间，使用默认值",
			zap.String("task_id", task.ID.String()),
			zap.Duration("default_timeout", r.timeout))
	}

	// 解析脚本配置
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析脚本配置失败: %w", err)
	}

	// 验证配置
	if err := r.validateConfig(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 替换环境变量（URL、文件路径等）
	if err := r.replaceVariables(); err != nil {
		return fmt.Errorf("环境变量替换失败: %w", err)
	}

	return nil
}

// validateConfig 验证脚本配置
func (r *ScriptRunner) validateConfig() error {
	// 1. 验证语言
	validLanguages := []string{"python", "nodejs", "node", "javascript", "js", "shell", "bash", "sh"}
	if !containsString(validLanguages, strings.ToLower(r.config.Language)) {
		return fmt.Errorf("不支持的语言: %s (支持: python, nodejs, shell)", r.config.Language)
	}

	// 标准化语言名称
	r.config.Language = r.normalizeLanguage(r.config.Language)

	// 2. 验证类型
	if r.config.Type != "file" && r.config.Type != "inline" {
		return fmt.Errorf("type 必须是 'file' 或 'inline'")
	}

	// 3. 根据类型验证必填字段
	if r.config.Type == "file" {
		// 文件模式：验证文件路径
		if strings.TrimSpace(r.config.File) == "" {
			return fmt.Errorf("文件模式下 file 字段不能为空")
		}
	} else if r.config.Type == "inline" {
		// 内联模式：验证代码内容
		if strings.TrimSpace(r.config.Code) == "" {
			return fmt.Errorf("内联模式下 code 字段不能为空")
		}
		// 验证代码长度
		if len(r.config.Code) > maxInlineCodeSize {
			return fmt.Errorf("内联代码长度超过限制: %d > %d (建议使用文件模式)",
				len(r.config.Code), maxInlineCodeSize)
		}
	}

	return nil
}

// normalizeLanguage 标准化语言名称
func (r *ScriptRunner) normalizeLanguage(lang string) string {
	lang = strings.ToLower(lang)
	switch lang {
	case "node", "javascript", "js":
		return "nodejs"
	case "bash", "sh":
		return "shell"
	default:
		return lang
	}
}

// replaceVariables 替换环境变量
func (r *ScriptRunner) replaceVariables() error {
	// 获取元数据
	metadata, err := r.task.GetMetadata()
	if err != nil {
		// 没有元数据也可以继续执行
		logger.Debug("获取元数据失败，跳过环境变量替换",
			zap.String("task_id", r.task.ID.String()),
			zap.Error(err))
		return nil
	}

	if len(metadata.Environment) == 0 {
		// 没有环境变量，跳过替换
		return nil
	}

	// 替换文件路径中的环境变量
	if r.config.Type == "file" {
		r.config.File = replaceString(r.config.File, metadata.Environment)
	}

	// 替换参数中的环境变量
	for i, arg := range r.config.Args {
		r.config.Args[i] = replaceString(arg, metadata.Environment)
	}

	// 替换解释器路径中的环境变量
	if r.config.Interpreter != "" {
		r.config.Interpreter = replaceString(r.config.Interpreter, metadata.Environment)
	}

	return nil
}

// Execute 执行脚本
func (r *ScriptRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	r.mutex.Lock()
	r.status = core.StatusRunning
	startTime := time.Now()
	r.mutex.Unlock()

	// 创建超时上下文
	execCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// 准备脚本路径
	scriptPath, cleanup, err := r.prepareScript()
	if err != nil {
		r.sendLog(logChan, fmt.Sprintf("准备脚本失败: %v", err))
		return r.buildErrorResult(err, startTime), err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// 获取解释器路径
	interpreter := r.getInterpreter()
	r.sendLog(logChan, fmt.Sprintf("使用解释器: %s\n", interpreter))
	r.sendLog(logChan, fmt.Sprintf("脚本路径: %s\n", scriptPath))

	// 构建执行命令
	// 格式：<interpreter> <scriptPath> <args...>
	cmdArgs := append([]string{scriptPath}, r.config.Args...)
	cmd := exec.CommandContext(execCtx, interpreter, cmdArgs...)
	r.cmd = cmd

	// 设置工作目录
	workingDir := r.getWorkingDir()
	if workingDir != "" {
		cmd.Dir = workingDir
		r.sendLog(logChan, fmt.Sprintf("工作目录: %s\n", workingDir))
	}

	// 设置环境变量
	cmd.Env = r.getEnvironment()

	// 设置进程组（用于信号处理）
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // 创建新的进程组
	}

	// 准备输出缓冲区
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 发送开始日志
	argsStr := strings.Join(r.config.Args, " ")
	if argsStr != "" {
		r.sendLog(logChan, fmt.Sprintf("执行命令: %s %s %s\n", interpreter, scriptPath, argsStr))
	} else {
		r.sendLog(logChan, fmt.Sprintf("执行命令: %s %s\n", interpreter, scriptPath))
	}

	// 执行命令
	err = cmd.Run()
	duration := time.Since(startTime)

	// 处理执行结果
	output := stdout.String() + stderr.String()

	if err != nil {
		// 执行失败
		r.sendLog(logChan, fmt.Sprintf("执行失败: %v\n", err))
		r.sendLog(logChan, fmt.Sprintf("标准输出:\n%s", stdout.String()))
		r.sendLog(logChan, fmt.Sprintf("标准错误:\n%s", stderr.String()))

		// 检查是否是超时
		if execCtx.Err() == context.DeadlineExceeded {
			timeoutErr := fmt.Errorf("执行超时: %v\n", r.timeout)
			return r.buildErrorResult(timeoutErr, startTime), timeoutErr
		}

		// 检查是否是用户停止
		if r.stopSignalType != "" {
			stopErr := fmt.Errorf("任务被%s信号停止\n", r.stopSignalType)
			return r.buildErrorResult(stopErr, startTime), stopErr
		}

		return r.buildErrorResult(err, startTime), err
	}

	// 执行成功
	r.sendLog(logChan, fmt.Sprintf("执行成功，耗时: %v\n", duration))
	r.sendLog(logChan, fmt.Sprintf("输出:\n%s\n", output))

	endTime := time.Now()
	r.mutex.Lock()
	r.status = core.StatusSuccess
	r.result = &core.Result{
		Status:     core.StatusSuccess,
		Output:     output,
		ExecuteLog: output,
		Error:      "",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
		ExitCode:   0,
	}
	r.mutex.Unlock()

	return r.result, nil
}

// prepareScript 准备脚本文件
//
// 返回：脚本路径、清理函数、错误
func (r *ScriptRunner) prepareScript() (string, func(), error) {
	if r.config.Type == "file" {
		// 文件模式：验证文件存在
		absPath, err := filepath.Abs(r.config.File)
		if err != nil {
			return "", nil, fmt.Errorf("解析文件路径失败: %w", err)
		}

		// 验证文件是否存在
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return "", nil, fmt.Errorf("脚本文件不存在: %s", absPath)
		}

		// 安全检查：验证文件路径是否在白名单中
		if !r.isPathAllowed(absPath) {
			return "", nil, fmt.Errorf("脚本文件路径不在白名单中: %s (允许的目录: %v)",
				absPath, allowedScriptDirs)
		}

		return absPath, nil, nil
	}

	// 内联模式：创建临时文件
	tmpFile, err := r.createTempScript(r.config.Code)
	if err != nil {
		return "", nil, fmt.Errorf("创建临时脚本失败: %w", err)
	}

	// 清理函数：删除临时文件
	cleanup := func() {
		if err := os.Remove(tmpFile); err != nil {
			logger.Warn("删除临时脚本文件失败",
				zap.String("file", tmpFile),
				zap.Error(err))
		}
	}

	return tmpFile, cleanup, nil
}

// createTempScript 创建临时脚本文件（内联模式）
func (r *ScriptRunner) createTempScript(code string) (string, error) {
	// 获取脚本文件扩展名
	ext := r.getScriptExtension()

	// 生成临时文件名：cronjob_script_<taskid>_<timestamp>.<ext>
	tmpFileName := fmt.Sprintf("cronjob_script_%s_%d%s",
		r.task.ID.String()[:8],
		time.Now().Unix(),
		ext,
	)

	// 创建临时文件路径
	tmpFile := filepath.Join(os.TempDir(), tmpFileName)

	// 写入脚本内容
	// 权限：0755 (所有者可读写执行，组和其他人可读执行)
	if err := os.WriteFile(tmpFile, []byte(code), 0755); err != nil {
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}

	logger.Debug("创建临时脚本文件",
		zap.String("task_id", r.task.ID.String()),
		zap.String("file", tmpFile),
		zap.Int("size", len(code)))

	r.tempFile = tmpFile
	return tmpFile, nil
}

// getScriptExtension 获取脚本文件扩展名
func (r *ScriptRunner) getScriptExtension() string {
	switch r.config.Language {
	case "python":
		return ".py"
	case "nodejs":
		return ".js"
	case "shell":
		return ".sh"
	default:
		return ".txt"
	}
}

// getInterpreter 获取解释器路径
func (r *ScriptRunner) getInterpreter() string {
	// 如果用户指定了解释器，直接使用
	if r.config.Interpreter != "" {
		return r.config.Interpreter
	}

	// 否则根据语言类型返回默认解释器
	switch r.config.Language {
	case "python":
		return "python3"
	case "nodejs":
		return "node"
	case "shell":
		return "/bin/bash"
	default:
		return r.config.Language
	}
}

// isPathAllowed 检查文件路径是否在白名单中
func (r *ScriptRunner) isPathAllowed(absPath string) bool {
	// 如果白名单为空，允许所有路径（不推荐，仅用于开发环境）
	if len(allowedScriptDirs) == 0 {
		logger.Warn("脚本文件路径白名单为空，允许所有路径（不安全）",
			zap.String("file", absPath))
		return true
	}

	// 检查路径是否在白名单目录中
	for _, allowedDir := range allowedScriptDirs {
		if strings.HasPrefix(absPath, allowedDir) {
			return true
		}
	}

	return false
}

// getWorkingDir 获取工作目录（从 metadata 读取）
func (r *ScriptRunner) getWorkingDir() string {
	metadata, err := r.task.GetMetadata()
	if err != nil {
		return ""
	}
	return metadata.WorkingDir
}

// getEnvironment 获取环境变量（合并系统环境变量和 metadata 中的环境变量）
func (r *ScriptRunner) getEnvironment() []string {
	// 从系统继承环境变量
	env := os.Environ()

	// 从 metadata 读取自定义环境变量
	metadata, err := r.task.GetMetadata()
	if err != nil {
		return env
	}

	// 添加自定义环境变量
	for key, value := range metadata.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// Stop 停止任务执行（优雅停止 - SIGTERM）
func (r *ScriptRunner) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return fmt.Errorf("任务未运行")
	}

	r.stopSignalType = "SIGTERM"

	// 发送 SIGTERM 信号到进程组
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		// 无法获取进程组，直接向进程发送信号
		return r.cmd.Process.Signal(syscall.SIGTERM)
	}

	// 向进程组发送信号（负数表示进程组）
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// Kill 强制终止任务（SIGKILL）
func (r *ScriptRunner) Kill() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return fmt.Errorf("任务未运行")
	}

	r.stopSignalType = "SIGKILL"

	// 发送 SIGKILL 信号到进程组
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		// 无法获取进程组，直接向进程发送信号
		return r.cmd.Process.Kill()
	}

	// 向进程组发送信号
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// GetStatus 获取当前状态
func (r *ScriptRunner) GetStatus() core.Status {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.status
}

// GetResult 获取执行结果
func (r *ScriptRunner) GetResult() *core.Result {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.result
}

// Cleanup 清理资源
func (r *ScriptRunner) Cleanup() error {
	// 清理临时文件（如果存在）
	if r.tempFile != "" {
		if err := os.Remove(r.tempFile); err != nil && !os.IsNotExist(err) {
			logger.Warn("清理临时脚本文件失败",
				zap.String("file", r.tempFile),
				zap.Error(err))
			return err
		}
		logger.Debug("清理临时脚本文件成功",
			zap.String("file", r.tempFile))
	}
	return nil
}

// ============ 辅助方法 ============

// buildErrorResult 构建错误结果
func (r *ScriptRunner) buildErrorResult(err error, startTime time.Time) *core.Result {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	endTime := time.Now()
	r.status = core.StatusFailed
	r.result = &core.Result{
		Status:     core.StatusFailed,
		Output:     "",
		ExecuteLog: "",
		Error:      err.Error(),
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime).Milliseconds(),
		ExitCode:   1,
	}
	return r.result
}

// sendLog 发送日志到通道
func (r *ScriptRunner) sendLog(logChan chan<- string, message string) {
	if logChan != nil {
		select {
		case logChan <- message:
		default:
			// 通道已满，跳过（避免阻塞）
		}
	}
}

// replaceString 替换字符串中的环境变量 ${VAR_NAME}
func replaceString(s string, env map[string]string) string {
	for key, value := range env {
		placeholder := fmt.Sprintf("${%s}", key)
		s = strings.ReplaceAll(s, placeholder, value)
	}
	return s
}

// containsString 检查切片中是否包含某个元素
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// init 注册 ScriptRunner（在 register.go 中调用）
func init() {
	// 从环境变量读取白名单配置
	if dirs := os.Getenv("ALLOWED_SCRIPT_DIRS"); dirs != "" {
		allowedScriptDirs = strings.Split(dirs, ";")
		logger.Info("从环境变量加载脚本文件路径白名单",
			zap.Strings("dirs", allowedScriptDirs))
	}
}
