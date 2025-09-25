package core

import (
	"time"

	"github.com/google/uuid"
)

// 任务状态常量
const (
	TaskStatusPending  = "pending"  // 待执行
	TaskStatusRunning  = "running"  // 运行中
	TaskStatusSuccess  = "success"  // 执行成功
	TaskStatusFailed   = "failed"   // 执行失败
	TaskStatusError    = "error"    // 执行错误
	TaskStatusTimeout  = "timeout"  // 执行超时
	TaskStatusCanceled = "canceled" // 已取消
	TaskStatusRetrying = "retrying" // 重试中
)

// Task 任务
type Task struct {
	ID           uuid.UUID  `json:"id"`                    // 任务ID
	Project      string     `json:"project"`               // 所属项目, 默认为default
	Category     string     `json:"category"`              // 任务类型
	CronJob      *uuid.UUID `json:"cronjob"`               // 归属的CronJob
	Name         string     `json:"name"`                  // 任务名称
	IsGroup      bool       `json:"is_group"`              // 是否为任务组
	TaskOrder    int        `json:"task_order"`            // 任务组内的顺序
	Previous     *uuid.UUID `json:"previous"`              // 上一个任务ID
	Next         *uuid.UUID `json:"next"`                  // 下一个任务ID
	Command      string     `json:"command"`               // 任务命令
	Args         string     `json:"args"`                  // 任务参数
	Description  string     `json:"description"`           // 任务描述
	TimePlan     time.Time  `json:"time_plan"`             // 计划执行时间
	TimeoutAt    time.Time  `json:"timeout_at"`            // 超时时间
	TimeStart    *time.Time `json:"time_start"`            // 开始执行时间
	TimeEnd      *time.Time `json:"time_end"`              // 结束执行时间
	Status       string     `json:"status"`                // 执行状态
	Output       string     `json:"output"`                // 输出结果
	SaveLog      bool       `json:"save_log"`              // 是否保存日志
	RetryCount   int        `json:"retry_count"`           // 已重试次数
	MaxRetry     int        `json:"max_retry"`             // 最大重试次数
	WorkerID     *uuid.UUID `json:"worker_id,omitempty"`   // 执行任务的工作节点ID
	WorkerName   string     `json:"worker_name,omitempty"` // 执行任务的工作节点名称
	IsStandalone bool       `json:"is_standalone"`         // 是否为独立任务，CronJob产生的就不是独立任务
	Timeout      int        `json:"timeout"`               // 任务超时时间，单位秒
}
