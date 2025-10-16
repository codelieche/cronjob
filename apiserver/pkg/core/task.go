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
	TaskStatusCanceled = "canceled" // 已取消 - 任务被手动取消（通常用于pending状态）
	TaskStatusStopped  = "stopped"  // 🔥 已停止 - 任务被用户主动停止（running状态被stop/kill）
	TaskStatusRetrying = "retrying" // 重试中 - 任务正在重试执行
)

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
	ID             uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                                                                                                                                     // 任务唯一标识
	TeamID         *uuid.UUID      `gorm:"size:256;index;index:idx_team_status_created,priority:1;index:idx_team_deleted,priority:1" json:"team_id"`                                                          // 团队ID，用于多租户隔离（复合索引：team_id+status+created_at, team_id+deleted_at）
	Project        string          `gorm:"size:128;index:idx_project;default:default" json:"project"`                                                                                                         // 所属项目，用于任务分组管理
	Category       string          `gorm:"size:128;index:idx_category;default:default" json:"category"`                                                                                                       // 任务分类，用于任务类型管理
	CronJob        *uuid.UUID      `gorm:"size:256;index:idx_cronjob;index:idx_cronjob_created,priority:1;index:idx_cronjob_team_deleted,priority:1;column:cronjob;" json:"cronjob"`                          // 关联的定时任务ID，独立任务为nil（复合索引：cronjob+created_at, cronjob+team_id+deleted_at）
	Workflow       *uuid.UUID      `gorm:"size:256;index:idx_workflow;index:idx_workflow_created,priority:1;column:workflow;" json:"workflow"`                                                                // 🔥 关联的工作流ID（冗余字段，提升查询性能），非工作流任务为nil（复合索引：workflow+created_at）
	WorkflowExecID *uuid.UUID      `gorm:"size:256;index:idx_workflow_exec;index:idx_workflow_exec_order,priority:1;column:workflow_exec_id;" json:"workflow_exec_id"`                                        // 🔥 关联的工作流执行实例ID，非工作流任务为nil（复合索引：workflow_exec_id+step_order）
	StepOrder      int             `gorm:"type:int;default:0;index:idx_workflow_exec_order,priority:2" json:"step_order"`                                                                                     // 🔥 工作流步骤序号（从1开始），非工作流任务为0（复合索引：workflow_exec_id+step_order）
	Name           string          `gorm:"size:256;index:idx_name" json:"name"`                                                                                                                               // 任务名称，通常包含时间戳
	IsGroup        *bool           `gorm:"type:boolean;default:false" json:"is_group"`                                                                                                                        // 是否为任务组，支持任务链式执行
	TaskOrder      int             `gorm:"type:int;default:0" json:"task_order"`                                                                                                                              // 任务组内的执行顺序
	Previous       *uuid.UUID      `gorm:"size:256;index:idx_previous" json:"previous"`                                                                                                                       // 前置任务ID，用于任务链
	Next           *uuid.UUID      `gorm:"size:256;index:idx_next" json:"next"`                                                                                                                               // 后续任务ID，用于任务链
	Command        string          `gorm:"size:512" json:"command"`                                                                                                                                           // 要执行的命令
	Args           string          `gorm:"type:text" json:"args"`                                                                                                                                             // 命令参数，JSON格式，支持大文本（如脚本代码）
	Description    string          `gorm:"size:512" json:"description"`                                                                                                                                       // 任务描述
	TimePlan       time.Time       `gorm:"column:time_plan;index:idx_tasks_pending_check,priority:2" json:"time_plan"`                                                                                        // 计划执行时间
	TimeoutAt      time.Time       `gorm:"column:timeout_at;index:idx_tasks_timeout_check,priority:2;index:idx_tasks_pending_check,priority:3" json:"timeout_at"`                                             // 任务超时时间点
	TimeStart      *time.Time      `gorm:"column:time_start" json:"time_start"`                                                                                                                               // 实际开始执行时间
	TimeEnd        *time.Time      `gorm:"column:time_end" json:"time_end"`                                                                                                                                   // 实际结束执行时间
	Status         string          `gorm:"size:40;index:idx_status;index:idx_tasks_timeout_check,priority:1;index:idx_tasks_pending_check,priority:1;index:idx_team_status_created,priority:2" json:"status"` // 当前执行状态（复合索引：team_id+status+created_at）
	Output         string          `gorm:"type:text" json:"output"`                                                                                                                                           // 任务执行输出（JSON格式），支持结构化数据
	SaveLog        *bool           `gorm:"type:boolean;default:true" json:"save_log"`                                                                                                                         // 是否保存执行日志
	RetryCount     int             `gorm:"type:int;default:0;index:idx_retry_count" json:"retry_count"`                                                                                                       // 当前重试次数（添加索引）
	MaxRetry       int             `gorm:"type:int;default:0" json:"max_retry"`                                                                                                                               // 最大重试次数（从CronJob继承）
	WorkerID       *uuid.UUID      `gorm:"size:256;index" json:"worker_id,omitempty"`                                                                                                                         // 执行此任务的Worker节点ID
	WorkerName     string          `gorm:"size:256;" json:"worker_name,omitempty"`                                                                                                                            // 执行此任务的Worker节点名称
	IsStandalone   *bool           `gorm:"type:boolean;default:false" json:"is_standalone"`                                                                                                                   // 是否为独立任务（非CronJob产生）
	Timeout        int             `gorm:"type:int;default:0" json:"timeout"`                                                                                                                                 // 超时时间（秒），0表示不限制
	Metadata       json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`                                                                                                                    // 任务元数据，存储执行环境、Worker配置等信息

	// 🔥 自动重试相关字段
	FailureReason string         `gorm:"size:256;index:idx_failure_reason" json:"failure_reason"`                                                                          // 失败原因分类（timeout/worker_error/network_error等）
	Retryable     *bool          `gorm:"type:boolean;index:idx_retryable" json:"retryable"`                                                                                // 是否可重试（从CronJob继承或Worker判断）
	NextRetryTime *time.Time     `gorm:"index:idx_next_retry_time" json:"next_retry_time"`                                                                                 // 下次重试时间（ApiServer计算）
	IsRetry       *bool          `gorm:"type:boolean;default:false;index:idx_is_retry" json:"is_retry"`                                                                    // 🔥 是否是重试任务（重试任务的ParentTask ID存储在Metadata.parent_task中）
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime;index:idx_team_status_created,priority:3;index:idx_cronjob_created,priority:2" json:"created_at"` // 任务创建时间（复合索引：team_id+status+created_at 和 cronjob+created_at）
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`                                                                               // 任务最后更新时间
	DeletedAt     gorm.DeletedAt `gorm:"index;index:idx_team_deleted,priority:2;index:idx_cronjob_team_deleted,priority:3" json:"-"`                                       // 软删除时间（复合索引：team_id+deleted_at, cronjob+team_id+deleted_at）
	Deleted       *bool          `gorm:"type:boolean;default:false" json:"deleted" form:"deleted"`                                                                         // 软删除标记
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
//
// 将JSON格式的Metadata字段解析为Metadata结构体
// 使用统一的 Metadata 结构（6 个字段）
//
// 返回：
//   - 解析后的 Metadata 结构体
//   - 解析错误（如果有）
func (t *Task) GetMetadata() (*Metadata, error) {
	return ParseMetadata(t.Metadata)
}

// SetMetadata 设置元数据
//
// 将 Metadata 结构体序列化为JSON并存储到Metadata字段
// 使用统一的 Metadata 结构（6 个字段）
//
// 参数：
//   - metadata: Metadata 结构体
//
// 返回：
//   - 序列化错误（如果有）
func (t *Task) SetMetadata(metadata *Metadata) error {
	data, err := SerializeMetadata(metadata)
	if err != nil {
		return err
	}
	t.Metadata = data
	return nil
}

// IsWorkflowTask 判断是否是工作流任务
//
// 返回：
//   - true: 是工作流任务（WorkflowExecID不为nil）
//   - false: 不是工作流任务
func (t *Task) IsWorkflowTask() bool {
	return t.WorkflowExecID != nil
}

// InheritMetadataFromCronJob 从CronJob继承元数据（精简版）
//
// 将CronJob的元数据复制到Task中，支持运行时覆盖特定字段
// 使用统一的 Metadata 结构和 MergeMetadata 函数
//
// 参数：
//   - cronJob: 父级 CronJob（如果是独立任务则为 nil）
//   - overrides: 运行时覆盖配置（可选）
//
// 返回：
//   - 设置元数据错误（如果有）
//
// 示例：
//
//	// 场景 1：普通 CronJob Task（不覆盖）
//	task.InheritMetadataFromCronJob(cronJob, nil)
//
//	// 场景 2：运行时覆盖环境变量
//	overrides := &Metadata{
//	    Environment: map[string]string{"DEBUG": "true"},
//	}
//	task.InheritMetadataFromCronJob(cronJob, overrides)
//
//	// 场景 3：独立任务（无 CronJob）
//	task.InheritMetadataFromCronJob(nil, &Metadata{
//	    WorkingDir: "/data/custom",
//	    Priority: 8,
//	})
func (t *Task) InheritMetadataFromCronJob(cronJob *CronJob, overrides *Metadata) error {
	// 如果没有 CronJob，直接设置 overrides
	if cronJob == nil {
		if overrides != nil {
			return t.SetMetadata(overrides)
		}
		return nil
	}

	// 获取 CronJob 的元数据
	cronJobMetadata, err := cronJob.GetMetadata()
	if err != nil {
		return err
	}

	// 🔥 使用统一的 MergeMetadata 函数（locked=false，允许覆盖）
	finalMetadata := MergeMetadata(cronJobMetadata, overrides, false)

	// 设置合并后的元数据
	return t.SetMetadata(finalMetadata)
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

	// GetNeedRetryTasks 获取需要重试的任务
	// 查询条件：
	//   - status IN (failed, error) - 不包括timeout
	//   - is_retry = false - 不是重试任务
	//   - retryable = true - 可重试
	//   - next_retry_time IS NOT NULL AND next_retry_time <= now - 已到重试时间
	//   - retry_count < max_retry - 未达到最大重试次数
	//   - max_retry > 0 - 配置了重试
	GetNeedRetryTasks(ctx context.Context, limit int) ([]*Task, error)
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

	// Cancel 取消待执行任务
	// 使用分布式锁确保并发安全，只能取消pending状态的任务
	Cancel(ctx context.Context, id string) (*Task, error)
}
