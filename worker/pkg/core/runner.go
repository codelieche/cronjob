package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Runner 任务执行器接口
//
// 定义了所有任务执行器必须实现的核心方法
// 支持任务解析、执行、生命周期管理等功能
type Runner interface {
	// ParseArgs 解析任务参数和配置
	// task: 完整的任务对象，包含命令、参数、元数据等所有信息
	// 返回解析错误，如果任务配置不正确
	ParseArgs(task *Task) error

	// Execute 执行任务
	// ctx: 上下文，支持取消和超时控制
	// logChan: 日志通道，用于实时输出执行日志
	// 返回执行结果和错误信息
	Execute(ctx context.Context, logChan chan<- string) (*Result, error)

	// Stop 停止任务执行
	// 发送SIGTERM信号，允许任务优雅退出
	// 返回停止操作的错误信息
	Stop() error

	// Kill 强制终止任务执行
	// 发送SIGKILL信号，立即终止任务
	// 返回终止操作的错误信息
	Kill() error

	// GetStatus 获取当前执行状态
	// 返回任务的当前状态
	GetStatus() Status

	// GetResult 获取执行结果
	// 返回任务的执行结果，如果任务未完成则返回nil
	GetResult() *Result

	// Cleanup 清理资源
	// 释放Runner占用的所有资源
	// 在任务完成后必须调用此方法
	Cleanup() error
}

// Status 任务执行状态
type Status string

const (
	StatusPending  Status = "pending"  // 等待执行（任务已创建，等待开始执行）
	StatusRunning  Status = "running"  // 正在执行（任务正在运行中，进程已启动）
	StatusSuccess  Status = "success"  // 执行成功（任务完成且返回成功状态码）
	StatusFailed   Status = "failed"   // 执行失败（任务完成但返回失败状态码）
	StatusTimeout  Status = "timeout"  // 执行超时（任务因超时而终止）
	StatusCanceled Status = "canceled" // 已取消（任务被手动取消，通常用于pending状态）
	StatusStopped  Status = "stopped"  // 🔥 已停止（任务被用户主动停止，running状态被stop/kill）
	StatusError    Status = "error"    // 执行错误（任务执行过程中发生异常）
)

// Result 任务执行结果
type Result struct {
	Status     Status    `json:"status"`      // 执行状态
	Output     string    `json:"output"`      // 执行输出（用于后续任务取数据）
	ExecuteLog string    `json:"execute_log"` // 执行日志（用于显示给用户）
	Error      string    `json:"error"`       // 错误信息
	StartTime  time.Time `json:"start_time"`  // 开始时间
	EndTime    time.Time `json:"end_time"`    // 结束时间
	Duration   int64     `json:"duration"`    // 执行时长(毫秒)
	ExitCode   int       `json:"exit_code"`   // 退出码
}

// RunnerFactory Runner工厂函数
// 用于创建Runner实例
type RunnerFactory func() Runner

// RunnerRegistry Runner注册表
// 管理所有可用的Runner类型
type RunnerRegistry struct {
	runners map[string]RunnerFactory
	mutex   sync.RWMutex
}

// NewRunnerRegistry 创建新的Runner注册表
func NewRunnerRegistry() *RunnerRegistry {
	return &RunnerRegistry{
		runners: make(map[string]RunnerFactory),
	}
}

// Register 注册Runner工厂
// category: Runner类型标识
// factory: Runner工厂函数
func (r *RunnerRegistry) Register(category string, factory RunnerFactory) {
	category = strings.ToLower(strings.TrimSpace(category))
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.runners[category] = factory
}

// Create 创建Runner实例
// category: Runner类型标识
// 返回Runner实例和错误信息
func (r *RunnerRegistry) Create(category string) (Runner, error) {
	if category == "" {
		category = "default"
	}
	category = strings.ToLower(strings.TrimSpace(category))

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	factory, exists := r.runners[category]
	if !exists {
		return nil, fmt.Errorf("未找到Runner类型: %s", category)
	}

	return factory(), nil
}

// List 列出所有已注册的Runner类型
func (r *RunnerRegistry) List() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	categories := make([]string, 0, len(r.runners))
	for category := range r.runners {
		categories = append(categories, category)
	}

	return categories
}

// 全局Runner注册表实例
var DefaultRegistry = NewRunnerRegistry()

// RegisterRunner 注册Runner到默认注册表
func RegisterRunner(category string, factory RunnerFactory) {
	DefaultRegistry.Register(category, factory)
}

// CreateRunner 从默认注册表创建Runner
func CreateRunner(category string) (Runner, error) {
	return DefaultRegistry.Create(category)
}

// ListRunners 列出默认注册表中的所有Runner类型
func ListRunners() []string {
	return DefaultRegistry.List()
}
