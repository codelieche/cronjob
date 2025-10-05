package core

import (
	"encoding/json"
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
	TaskStatusCanceled = "canceled" // 已取消（通常用于pending状态）
	TaskStatusStopped  = "stopped"  // 🔥 已停止（running状态被stop/kill）
	TaskStatusRetrying = "retrying" // 重试中
)

// TaskMetadata 任务元数据
//
// 定义任务的执行环境和配置信息，包括：
// - 执行环境：工作目录、环境变量等
// - Worker配置：选择执行节点、节点标签等
// - 扩展配置：其他自定义配置信息
//
// 使用示例：
//
//	metadata := &TaskMetadata{
//	    WorkingDir: "/var/logs",
//	    Environment: map[string]string{
//	        "LOG_LEVEL": "INFO",
//	        "APP_NAME": "myapp",
//	    },
//	    WorkerSelect: []string{"worker-01", "worker-02"},
//	    Priority: 5,
//	}
type TaskMetadata struct {
	WorkingDir    string                 `json:"workingDir,omitempty"`     // 任务执行的工作目录（如：/var/logs）
	Environment   map[string]string      `json:"environment,omitempty"`    // 环境变量设置（如：{"LOG_LEVEL": "INFO"}）
	WorkerSelect  []string               `json:"worker_select,omitempty"`  // 可执行此任务的Worker节点名称列表，空表示所有Worker
	WorkerLabels  map[string]string      `json:"worker_labels,omitempty"`  // Worker节点标签选择器（如：{"env": "prod", "type": "web"}）
	Priority      int                    `json:"priority,omitempty"`       // 任务优先级（1-10，默认5，数字越大优先级越高）
	ResourceLimit map[string]string      `json:"resource_limit,omitempty"` // 资源限制配置（如：{"cpu": "1000m", "memory": "512Mi"}）
	Extensions    map[string]interface{} `json:"extensions,omitempty"`     // 扩展字段，用于存储其他自定义配置
}

// Task 任务
//
// 表示一个具体的任务实例，包含任务的所有信息：
// - 基本信息：ID、名称、描述等
// - 执行信息：命令、参数、超时等
// - 状态信息：当前状态、执行时间等
// - 配置信息：元数据、重试配置等
//
// 使用示例：
//
//	task := &Task{
//	    ID: uuid.New(),
//	    Name: "数据库备份",
//	    Command: "/usr/local/bin/backup.sh",
//	    Args: `["--database", "production"]`,
//	    Timeout: 3600, // 1小时
//	}
type Task struct {
	ID           uuid.UUID       `json:"id"`                    // 任务唯一标识符
	Project      string          `json:"project"`               // 所属项目名称，默认为"default"
	Category     string          `json:"category"`              // 任务类型（如：command、script、docker）
	CronJob      *uuid.UUID      `json:"cronjob"`               // 归属的定时任务ID（如果是定时任务产生的）
	Name         string          `json:"name"`                  // 任务名称（用于显示和识别）
	IsGroup      *bool           `json:"is_group"`              // 是否为任务组（任务组可以包含多个子任务）
	TaskOrder    int             `json:"task_order"`            // 任务组内的执行顺序（从0开始）
	Previous     *uuid.UUID      `json:"previous"`              // 上一个任务的ID（用于任务链）
	Next         *uuid.UUID      `json:"next"`                  // 下一个任务的ID（用于任务链）
	Command      string          `json:"command"`               // 要执行的命令（如：ls、python、docker）
	Args         string          `json:"args"`                  // 命令参数（JSON数组格式，如：["-l", "-a"]）
	Description  string          `json:"description"`           // 任务描述（用于说明任务用途）
	TimePlan     time.Time       `json:"time_plan"`             // 计划执行时间（任务应该开始执行的时间）
	TimeoutAt    time.Time       `json:"timeout_at"`            // 超时时间（任务必须在此时间前完成）
	TimeStart    *time.Time      `json:"time_start"`            // 实际开始执行时间（任务真正开始的时间）
	TimeEnd      *time.Time      `json:"time_end"`              // 实际结束执行时间（任务完成的时间）
	Status       string          `json:"status"`                // 当前执行状态（pending、running、success等）
	Output       string          `json:"output"`                // 任务执行输出（命令的标准输出和错误输出）
	SaveLog      *bool           `json:"save_log"`              // 是否保存执行日志到文件（true=保存，false=不保存）
	RetryCount   int             `json:"retry_count"`           // 已重试次数（任务失败后自动重试的次数）
	MaxRetry     int             `json:"max_retry"`             // 最大重试次数（任务失败后最多重试几次）
	WorkerID     *uuid.UUID      `json:"worker_id,omitempty"`   // 执行任务的工作节点ID（哪个Worker执行了这个任务）
	WorkerName   string          `json:"worker_name,omitempty"` // 执行任务的工作节点名称（Worker的显示名称）
	IsStandalone *bool           `json:"is_standalone"`         // 是否为独立任务（true=独立任务，false=定时任务产生的任务）
	Timeout      int             `json:"timeout"`               // 任务超时时间（单位：秒，超过此时间任务将被终止）
	Metadata     json.RawMessage `json:"metadata"`              // 任务元数据（JSON格式，包含工作目录、环境变量等配置）
}

// GetMetadata 获取解析后的元数据
//
// 将JSON格式的Metadata字段解析为TaskMetadata结构体，便于使用
//
// 返回值:
//   - *TaskMetadata: 解析后的元数据对象，如果解析失败返回nil
//   - error: 解析错误，如果JSON格式不正确
//
// 使用示例:
//
//	metadata, err := task.GetMetadata()
//	if err != nil {
//	    log.Printf("解析元数据失败: %v", err)
//	    return
//	}
//	if metadata.WorkingDir != "" {
//	    // 使用工作目录
//	}
func (t *Task) GetMetadata() (*TaskMetadata, error) {
	if len(t.Metadata) == 0 {
		return &TaskMetadata{}, nil
	}

	var metadata TaskMetadata
	if err := json.Unmarshal(t.Metadata, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}
