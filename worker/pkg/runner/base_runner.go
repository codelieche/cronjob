package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// BaseRunner 基础 Runner 结构
//
// 设计原则：
//   - 公共字段：大写导出，子类可以直接访问（符合 Go 嵌入的惯用法）
//   - 私有字段：小写私有，只有 mutex 保持私有以保证并发安全
//   - 辅助方法：提供复杂操作的辅助方法（如 GetWorkingDirectory）
//
// 提供所有 Runner 的公共功能：
//   - 任务对象管理
//   - 状态管理
//   - 结果管理
//   - 并发安全（读写锁）
//   - API Server 客户端注入
//   - Context 管理（取消和超时）
//   - 执行时间追踪
//   - 工作目录获取
//   - 日志发送
type BaseRunner struct {
	// 🔥 公共字段 - 子类可直接访问
	Task      *core.Task         // 任务对象
	Status    core.Status        // 当前状态
	Result    *core.Result       // 执行结果
	Apiserver core.Apiserver     // API Server 客户端
	Ctx       context.Context    // 执行上下文
	Cancel    context.CancelFunc // 取消函数
	StartTime time.Time          // 开始时间

	// 🔥 私有字段 - 保持封装
	mutex sync.RWMutex // 读写锁（子类通过 Lock/Unlock 方法访问）
}

// InitBase 初始化 BaseRunner
//
// 子类 Runner 应在构造函数中调用此方法
func (b *BaseRunner) InitBase() {
	b.Status = core.StatusPending
	b.Result = nil
}

// SetApiserver 注入 API Server 客户端（保留此方法以符合接口）
func (b *BaseRunner) SetApiserver(apiserver core.Apiserver) {
	b.Apiserver = apiserver
}

// GetStatus 获取当前状态（保留此方法以符合接口）
func (b *BaseRunner) GetStatus() core.Status {
	return b.Status
}

// GetResult 获取执行结果（保留此方法以符合接口）
func (b *BaseRunner) GetResult() *core.Result {
	return b.Result
}

// SendLog 发送日志到 channel（非阻塞）
//
// 使用 select 实现非阻塞发送，避免 channel 满时阻塞
func (b *BaseRunner) SendLog(logChan chan<- string, message string) {
	if logChan == nil {
		return
	}

	select {
	case logChan <- message:
		// 成功发送
	default:
		// channel 已满，丢弃日志
		// 注意：这里不应该阻塞，因为日志不应该影响任务执行
	}
}

// GetWorkingDirectory 获取任务工作目录
//
// 自动处理：
//  1. 从 task.Metadata 中读取自定义工作目录
//  2. 如果没有配置，使用默认目录
//  3. 自动去除路径两边的空格
//  4. 自动创建目录（如果不存在）
//  5. 验证路径是否为目录
//
// 返回：
//   - string: 工作目录路径
//   - error: 如果创建目录失败或路径不是目录
func (b *BaseRunner) GetWorkingDirectory() (string, error) {
	// 🔥 直接访问公共字段
	if b.Task == nil {
		return "", fmt.Errorf("任务对象未设置")
	}

	var workDir string

	// 1. 优先从 metadata 中读取自定义工作目录
	if len(b.Task.Metadata) > 0 {
		if metadata, err := b.Task.GetMetadata(); err == nil && metadata.WorkingDir != "" {
			// 去除前后空格，防止用户输入错误
			workDir = strings.TrimSpace(metadata.WorkingDir)
		}
	}

	// 2. 如果没有配置，使用默认目录
	if workDir == "" {
		workDir = b.getDefaultWorkingDirectory(b.Task)
	}

	// 3. 如果是空字符串或当前目录，直接返回
	if workDir == "" || workDir == "." {
		return workDir, nil
	}

	// 4. 转换为绝对路径（如果不是绝对路径）
	if !filepath.IsAbs(workDir) {
		absPath, err := filepath.Abs(workDir)
		if err != nil {
			return "", fmt.Errorf("无法解析工作目录路径 %s: %w", workDir, err)
		}
		workDir = absPath
	}

	// 5. 检查目录是否存在，不存在则创建
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		// 目录不存在，创建目录（权限 0755）
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return "", fmt.Errorf("无法创建工作目录 %s: %w", workDir, err)
		}

		// 记录日志
		logger.Debug("自动创建工作目录",
			zap.String("task_id", b.Task.ID.String()),
			zap.String("workDir", workDir))
	} else if err != nil {
		// 其他错误（权限问题等）
		return "", fmt.Errorf("无法访问工作目录 %s: %w", workDir, err)
	}

	// 6. 验证路径是否为目录
	if stat, err := os.Stat(workDir); err == nil {
		if !stat.IsDir() {
			return "", fmt.Errorf("工作目录路径 %s 不是一个目录", workDir)
		}
	}

	return workDir, nil
}

// getDefaultWorkingDirectory 生成默认工作目录
//
// 根据任务信息生成默认的工作目录路径：
//   - 如果任务有 CronJob，使用 {baseDir}/tasks/{cronjob_id}
//   - 否则使用 {baseDir}/tasks/{task_id}
func (b *BaseRunner) getDefaultWorkingDirectory(task *core.Task) string {
	baseDir := config.WorkerInstance.WorkingDir

	// 如果任务有 CronJob，使用 CronJob 的 ID
	if task.CronJob != nil {
		return filepath.Join(baseDir, "tasks", task.CronJob.String())
	}

	// 否则使用任务自己的 ID
	return filepath.Join(baseDir, "tasks", task.ID.String())
}

// Lock 获取写锁（供子类使用）
func (b *BaseRunner) Lock() {
	b.mutex.Lock()
}

// Unlock 释放写锁（供子类使用）
func (b *BaseRunner) Unlock() {
	b.mutex.Unlock()
}

// RLock 获取读锁（供子类使用）
func (b *BaseRunner) RLock() {
	b.mutex.RLock()
}

// RUnlock 释放读锁（供子类使用）
func (b *BaseRunner) RUnlock() {
	b.mutex.RUnlock()
}
