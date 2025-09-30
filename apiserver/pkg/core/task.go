// Package core 核心数据模型和接口定义
//
// 包含系统中所有核心业务实体的数据模型定义
// 以及相关的数据访问接口和服务接口
package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 任务执行状态常量定义
// 用于标识任务在生命周期中的不同状态
const (
	TaskStatusPending  = "pending"  // 待执行 - 任务已创建，等待调度执行
	TaskStatusRunning  = "running"  // 运行中 - 任务正在执行
	TaskStatusSuccess  = "success"  // 执行成功 - 任务执行完成且成功
	TaskStatusFailed   = "failed"   // 执行失败 - 任务执行完成但失败
	TaskStatusError    = "error"    // 执行错误 - 任务执行过程中发生错误
	TaskStatusTimeout  = "timeout"  // 执行超时 - 任务执行时间超过设定值
	TaskStatusCanceled = "canceled" // 已取消 - 任务被手动取消
	TaskStatusRetrying = "retrying" // 重试中 - 任务正在重试执行
)

// TaskMetadata 任务元数据
//
// 定义任务的执行环境和配置信息，继承自CronJob的元数据
// 支持运行时覆盖和扩展配置，包括：
// - 执行环境：工作目录、环境变量等
// - Worker配置：指定执行节点、节点标签等
// - 扩展配置：其他自定义配置信息
type TaskMetadata struct {
	WorkingDir    string                 `json:"workingDir,omitempty"`     // 任务执行的工作目录
	Environment   map[string]string      `json:"environment,omitempty"`    // 环境变量设置
	WorkerSelect  []string               `json:"worker_select,omitempty"`  // 可执行此任务的Worker节点名称列表，空表示所有Worker
	WorkerLabels  map[string]string      `json:"worker_labels,omitempty"`  // Worker节点标签选择器
	Priority      int                    `json:"priority,omitempty"`       // 任务优先级（1-10，默认5）
	ResourceLimit map[string]string      `json:"resource_limit,omitempty"` // 资源限制配置
	Extensions    map[string]interface{} `json:"extensions,omitempty"`     // 扩展字段，用于存储其他自定义配置
}

// Task 任务执行记录实体
//
// 记录每次任务执行的详细信息，包括：
// - 基本信息：任务名称、描述、归属项目等
// - 执行信息：命令、参数、执行时间等
// - 状态信息：执行状态、输出结果、重试次数等
// - 关联信息：所属的CronJob、执行的Worker等
// - 元数据信息：执行环境、Worker配置等（继承自CronJob或运行时指定）
//
// 这是CronJob的具体执行实例，每次调度都会创建一个新的Task
type Task struct {
	ID           uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                               // 任务唯一标识
	TeamID       *uuid.UUID      `gorm:"size:256;index" json:"team_id"`                               // 团队ID，用于多租户隔离
	Project      string          `gorm:"size:128;index:idx_project;default:default" json:"project"`   // 所属项目，用于任务分组管理
	Category     string          `gorm:"size:128;index:idx_category;default:default" json:"category"` // 任务分类，用于任务类型管理
	CronJob      *uuid.UUID      `gorm:"size:256;index:idx_cronjob;column:cronjob;" json:"cronjob"`   // 关联的定时任务ID，独立任务为nil
	Name         string          `gorm:"size:256;index:idx_name" json:"name"`                         // 任务名称，通常包含时间戳
	IsGroup      *bool           `gorm:"type:boolean;default:false" json:"is_group"`                  // 是否为任务组，支持任务链式执行
	TaskOrder    int             `gorm:"type:int;default:0" json:"task_order"`                        // 任务组内的执行顺序
	Previous     *uuid.UUID      `gorm:"size:256;index:idx_previous" json:"previous"`                 // 前置任务ID，用于任务链
	Next         *uuid.UUID      `gorm:"size:256;index:idx_next" json:"next"`                         // 后续任务ID，用于任务链
	Command      string          `gorm:"size:512" json:"command"`                                     // 要执行的命令
	Args         string          `gorm:"size:512" json:"args"`                                        // 命令参数，JSON格式
	Description  string          `gorm:"size:512" json:"description"`                                 // 任务描述
	TimePlan     time.Time       `gorm:"column:time_plan" json:"time_plan"`                           // 计划执行时间
	TimeoutAt    time.Time       `gorm:"column:timeout_at" json:"timeout_at"`                         // 任务超时时间点
	TimeStart    *time.Time      `gorm:"column:time_start" json:"time_start"`                         // 实际开始执行时间
	TimeEnd      *time.Time      `gorm:"column:time_end" json:"time_end"`                             // 实际结束执行时间
	Status       string          `gorm:"size:40;index:idx_status" json:"status"`                      // 当前执行状态
	Output       string          `gorm:"size:1024" json:"output"`                                     // 任务执行输出结果
	SaveLog      *bool           `gorm:"type:boolean;default:true" json:"save_log"`                   // 是否保存执行日志
	RetryCount   int             `gorm:"type:int;default:0" json:"retry_count"`                       // 当前重试次数
	MaxRetry     int             `gorm:"type:int;default:0" json:"max_retry"`                         // 最大重试次数
	WorkerID     *uuid.UUID      `gorm:"size:256;index" json:"worker_id,omitempty"`                   // 执行此任务的Worker节点ID
	WorkerName   string          `gorm:"size:256;" json:"worker_name,omitempty"`                      // 执行此任务的Worker节点名称
	IsStandalone *bool           `gorm:"type:boolean;default:false" json:"is_standalone"`             // 是否为独立任务（非CronJob产生）
	Timeout      int             `gorm:"type:int;default:0" json:"timeout"`                           // 超时时间（秒），0表示不限制
	Metadata     json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`              // 任务元数据，存储执行环境、Worker配置等信息
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`          // 任务创建时间
	UpdatedAt    time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`          // 任务最后更新时间
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`                                              // 软删除时间
	Deleted      *bool           `gorm:"type:boolean;default:false" json:"deleted" form:"deleted"`    // 软删除标记
}

// TableName 表名
func (Task) TableName() string {
	return "tasks"
}

// BeforeDelete 删除前设置deleted字段为True
// 同时执行删除操作的额外处理
func (m *Task) BeforeDelete(tx *gorm.DB) (err error) {
	// 设置Deleted字段为true
	trueValue := true
	m.Deleted = &trueValue

	return nil
}

// AfterDelete 钩子函数，在删除后执行
func (m *Task) AfterDelete(tx *gorm.DB) (err error) {
	// 这里可以添加删除后的处理逻辑
	return
}

// GetMetadata 获取解析后的元数据
// 将JSON格式的Metadata字段解析为TaskMetadata结构体
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

// SetMetadata 设置元数据
// 将TaskMetadata结构体序列化为JSON并存储到Metadata字段
func (t *Task) SetMetadata(metadata *TaskMetadata) error {
	if metadata == nil {
		t.Metadata = nil
		return nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	t.Metadata = data
	return nil
}

// InheritMetadataFromCronJob 从CronJob继承元数据
// 将CronJob的元数据复制到Task中，支持运行时覆盖特定字段
func (t *Task) InheritMetadataFromCronJob(cronJob *CronJob, overrides *TaskMetadata) error {
	if cronJob == nil {
		if overrides != nil {
			return t.SetMetadata(overrides)
		}
		return nil
	}

	// 获取CronJob的元数据
	cronJobMetadata, err := cronJob.GetMetadata()
	if err != nil {
		return err
	}

	// 创建Task元数据，继承CronJob的配置
	taskMetadata := &TaskMetadata{
		WorkingDir:    cronJobMetadata.WorkingDir,
		Environment:   make(map[string]string),
		WorkerSelect:  make([]string, len(cronJobMetadata.WorkerSelect)),
		WorkerLabels:  make(map[string]string),
		Priority:      cronJobMetadata.Priority,
		ResourceLimit: make(map[string]string),
		Extensions:    make(map[string]interface{}),
	}

	// 复制WorkerSelect切片
	copy(taskMetadata.WorkerSelect, cronJobMetadata.WorkerSelect)

	// 复制map类型字段
	for k, v := range cronJobMetadata.Environment {
		taskMetadata.Environment[k] = v
	}
	for k, v := range cronJobMetadata.WorkerLabels {
		taskMetadata.WorkerLabels[k] = v
	}
	for k, v := range cronJobMetadata.ResourceLimit {
		taskMetadata.ResourceLimit[k] = v
	}
	for k, v := range cronJobMetadata.Extensions {
		taskMetadata.Extensions[k] = v
	}

	// 应用运行时覆盖
	if overrides != nil {
		if overrides.WorkingDir != "" {
			taskMetadata.WorkingDir = overrides.WorkingDir
		}
		if len(overrides.WorkerSelect) > 0 {
			taskMetadata.WorkerSelect = make([]string, len(overrides.WorkerSelect))
			copy(taskMetadata.WorkerSelect, overrides.WorkerSelect)
		}
		if overrides.Priority != 0 {
			taskMetadata.Priority = overrides.Priority
		}

		// 合并环境变量（运行时覆盖优先）
		for k, v := range overrides.Environment {
			taskMetadata.Environment[k] = v
		}

		// 合并Worker标签（运行时覆盖优先）
		for k, v := range overrides.WorkerLabels {
			taskMetadata.WorkerLabels[k] = v
		}

		// 合并资源限制（运行时覆盖优先）
		for k, v := range overrides.ResourceLimit {
			taskMetadata.ResourceLimit[k] = v
		}

		// 合并扩展字段（运行时覆盖优先）
		for k, v := range overrides.Extensions {
			taskMetadata.Extensions[k] = v
		}
	}

	return t.SetMetadata(taskMetadata)
}

// TaskStore 任务存储接口
type TaskStore interface {
	// FindByID 根据ID获取任务
	FindByID(ctx context.Context, id uuid.UUID) (*Task, error)

	// Create 创建任务
	Create(ctx context.Context, obj *Task) (*Task, error)

	// Update 更新任务信息
	Update(ctx context.Context, obj *Task) (*Task, error)

	// Delete 删除任务
	Delete(ctx context.Context, obj *Task) error

	// DeleteByID 根据ID删除任务
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取任务列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (tasks []*Task, err error)

	// Count 统计任务数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// UpdateStatus 更新任务状态
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error

	// UpdateOutput 更新任务输出
	UpdateOutput(ctx context.Context, id uuid.UUID, output string) error

	// Patch 动态更新任务字段
	Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
}

// TaskService 任务服务接口
type TaskService interface {
	// FindByID 根据ID获取任务
	FindByID(ctx context.Context, id string) (*Task, error)

	// Create 创建任务
	Create(ctx context.Context, obj *Task) (*Task, error)

	// Update 更新任务信息
	Update(ctx context.Context, obj *Task) (*Task, error)

	// Delete 删除任务
	Delete(ctx context.Context, obj *Task) error

	// DeleteByID 根据ID删除任务
	DeleteByID(ctx context.Context, id string) error

	// List 获取任务列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (tasks []*Task, err error)

	// Count 统计任务数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// UpdateStatus 更新任务状态
	UpdateStatus(ctx context.Context, id string, status string) error

	// UpdateOutput 更新任务输出
	UpdateOutput(ctx context.Context, id string, output string) error

	// Patch 动态更新任务字段
	Patch(ctx context.Context, id string, updates map[string]interface{}) error
}
