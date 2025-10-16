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

// 工作流执行状态常量
const (
	WorkflowExecuteStatusPending  = "pending"  // 待执行 - 工作流已创建，等待执行
	WorkflowExecuteStatusRunning  = "running"  // 执行中 - 工作流正在执行
	WorkflowExecuteStatusSuccess  = "success"  // 执行成功 - 所有步骤成功完成
	WorkflowExecuteStatusFailed   = "failed"   // 执行失败 - 某个步骤失败
	WorkflowExecuteStatusCanceled = "canceled" // 已取消 - 用户主动取消执行
)

// WorkflowExecute 工作流执行实例
//
// 记录每次工作流执行的详细信息，包括：
// - 基本信息：关联的Workflow、触发方式、触发者等
// - 执行信息：开始时间、结束时间、执行状态等
// - 步骤统计：总步骤数、已完成步骤数、成功/失败数等
// - 环境锁定：锁定的Worker、工作目录等
// - 参数传递：Variables全局变量（用于步骤间参数传递）
//
// 这是Workflow的具体执行实例，每次执行都会创建一个新的WorkflowExecute
type WorkflowExecute struct {
	ID         uuid.UUID  `gorm:"size:256;primaryKey" json:"id"`                                                    // 执行实例唯一标识
	TeamID     *uuid.UUID `gorm:"size:256;index:idx_workflow_exec_team" json:"team_id"`                             // 团队ID，用于多租户隔离
	WorkflowID uuid.UUID  `gorm:"size:256;index:idx_workflow_exec_workflow,priority:1;not null" json:"workflow_id"` // 关联的工作流ID（复合索引：workflow_id+created_at）
	Project    string     `gorm:"size:128;index:idx_workflow_exec_project;default:default" json:"project"`          // 所属项目（从 Workflow 继承），用于分组和过滤

	// 触发信息
	TriggerType string     `gorm:"size:40;index" json:"trigger_type"` // 触发类型：manual（手动）、api（API调用）、webhook（Webhook）
	UserID      *uuid.UUID `gorm:"size:256" json:"user_id"`           // 触发者用户ID（手动触发时）
	Username    string     `gorm:"size:128" json:"username"`          // 触发者用户名

	// 执行状态
	Status    string     `gorm:"size:40;index:idx_workflow_exec_status" json:"status"` // 执行状态：pending/running/success/failed/canceled
	TimeStart *time.Time `gorm:"column:time_start" json:"time_start"`                  // 实际开始时间（第一个Task开始时）
	TimeEnd   *time.Time `gorm:"column:time_end" json:"time_end"`                      // 实际结束时间（所有Task完成时）

	// 步骤统计
	TotalSteps     int `gorm:"type:int;default:0" json:"total_steps"`     // 总步骤数
	CompletedSteps int `gorm:"type:int;default:0" json:"completed_steps"` // 已完成步骤数（success + failed）
	SuccessSteps   int `gorm:"type:int;default:0" json:"success_steps"`   // 成功步骤数
	FailedSteps    int `gorm:"type:int;default:0" json:"failed_steps"`    // 失败步骤数
	CurrentStep    int `gorm:"type:int;default:0" json:"current_step"`    // 当前执行的步骤序号（Order）

	// 环境锁定信息（第一个Task完成后锁定）
	LockedWorkerID   *uuid.UUID `gorm:"size:256" json:"locked_worker_id"`   // 锁定的Worker ID
	LockedWorkerName string     `gorm:"size:256" json:"locked_worker_name"` // 锁定的Worker名称
	LockedWorkingDir string     `gorm:"size:512" json:"locked_working_dir"` // 锁定的工作目录

	// ========== ⭐ 全局变量（Variables）- 参数传递核心 ==========
	Variables json.RawMessage `gorm:"type:json" json:"variables" swaggertype:"object"`
	// 存储工作流执行过程中的所有变量（键值对）
	// 结构示例：
	// {
	//   "branch": "develop",                 // 初始变量（用户传入）
	//   "deploy_env": "production",          // 初始变量
	//   "image_tag": "v1.2.3-abc123",       // Task 1 的输出
	//   "commit_sha": "abc123def456",        // Task 1 的输出
	//   "image_size": "125MB",               // Task 2 的输出
	//   "deploy_time": "2025-10-16T14:30:00" // Task N 的输出
	// }
	//
	// 用途：
	//   1. 存储用户传入的初始变量（initial_variables）
	//   2. 每个 Task 完成后，将其 Output 合并到 Variables
	//   3. 激活下一个 Task 时，用 Variables 中的值替换 Task.Args 中的 ${variable}
	//
	// 详见：[05-参数传递机制设计.md](../docs/workflow/05-参数传递机制设计.md)

	// 元数据
	Metadata json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"` // 元数据（继承自Workflow，可被覆盖）

	// 错误信息
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"` // 错误信息（执行失败时）

	// 时间戳字段
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime;index:idx_workflow_exec_workflow,priority:2;index:idx_workflow_exec_created" json:"created_at"` // 创建时间（复合索引：workflow_id+created_at）
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`                                                                             // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                                                                                                 // 软删除时间
	Deleted   *bool          `gorm:"type:boolean;default:false" json:"deleted"`                                                                                      // 软删除标记

	// 🔥 关联数据（不存储在数据库，仅用于API返回）
	Tasks []*Task `gorm:"-" json:"tasks,omitempty"` // 关联的任务列表（按 step_order 排序）
}

// TableName 表名
func (WorkflowExecute) TableName() string {
	return "workflow_executes"
}

// GetVariables 获取解析后的变量
//
// 将JSON格式的Variables字段解析为map
//
// 返回：
//   - 解析后的变量map
//   - 解析错误（如果有）
func (w *WorkflowExecute) GetVariables() (map[string]interface{}, error) {
	if len(w.Variables) == 0 {
		return make(map[string]interface{}), nil
	}

	var variables map[string]interface{}
	if err := json.Unmarshal(w.Variables, &variables); err != nil {
		return nil, err
	}
	return variables, nil
}

// SetVariables 设置变量
//
// 将map序列化为JSON并存储到Variables字段
//
// 参数：
//   - variables: 变量map
//
// 返回：
//   - 序列化错误（如果有）
func (w *WorkflowExecute) SetVariables(variables map[string]interface{}) error {
	data, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	w.Variables = data
	return nil
}

// MergeVariables 合并变量
//
// 将新变量合并到现有Variables中（新变量会覆盖同名旧变量）
//
// 参数：
//   - newVariables: 要合并的新变量
//
// 返回：
//   - 错误（如果有）
func (w *WorkflowExecute) MergeVariables(newVariables map[string]interface{}) error {
	// 1. 获取现有变量
	existingVariables, err := w.GetVariables()
	if err != nil {
		return err
	}

	// 2. 合并新变量
	for k, v := range newVariables {
		existingVariables[k] = v
	}

	// 3. 保存合并后的变量
	return w.SetVariables(existingVariables)
}

// GetMetadata 获取解析后的元数据
func (w *WorkflowExecute) GetMetadata() (*Metadata, error) {
	return ParseMetadata(w.Metadata)
}

// SetMetadata 设置元数据
func (w *WorkflowExecute) SetMetadata(metadata *Metadata) error {
	data, err := SerializeMetadata(metadata)
	if err != nil {
		return err
	}
	w.Metadata = data
	return nil
}

// UpdateStepStats 更新步骤统计信息
//
// 在Task完成后调用，更新步骤统计
//
// 参数：
//   - stepOrder: 步骤序号
//   - success: 是否成功
func (w *WorkflowExecute) UpdateStepStats(stepOrder int, success bool) {
	w.CompletedSteps++
	if success {
		w.SuccessSteps++
	} else {
		w.FailedSteps++
	}

	// 更新当前步骤
	if stepOrder > w.CurrentStep {
		w.CurrentStep = stepOrder
	}
}

// IsCompleted 判断是否已完成
//
// 返回：
//   - true: 已完成（success/failed/canceled）
//   - false: 未完成（pending/running）
func (w *WorkflowExecute) IsCompleted() bool {
	return w.Status == WorkflowExecuteStatusSuccess ||
		w.Status == WorkflowExecuteStatusFailed ||
		w.Status == WorkflowExecuteStatusCanceled
}

// CanCancel 判断是否可以取消
//
// 返回：
//   - true: 可以取消（pending/running）
//   - false: 不可以取消（success/failed/canceled）
func (w *WorkflowExecute) CanCancel() bool {
	return w.Status == WorkflowExecuteStatusPending ||
		w.Status == WorkflowExecuteStatusRunning
}

// BeforeCreate GORM钩子：创建前的处理
func (w *WorkflowExecute) BeforeCreate(tx *gorm.DB) error {
	// 设置ID
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}

	// 设置默认状态
	if w.Status == "" {
		w.Status = WorkflowExecuteStatusPending
	}

	// 初始化统计信息
	if w.TotalSteps == 0 {
		// TotalSteps应该在创建时设置
	}

	return nil
}

// BeforeDelete 删除前设置deleted字段为True
func (w *WorkflowExecute) BeforeDelete(tx *gorm.DB) error {
	// 设置Deleted字段为true
	trueValue := true
	w.Deleted = &trueValue
	return nil
}

// WorkflowExecuteStore 工作流执行数据存储接口
//
// 定义了工作流执行的所有数据访问操作
type WorkflowExecuteStore interface {
	// Create 创建工作流执行实例
	Create(ctx context.Context, execute *WorkflowExecute) error

	// Update 更新工作流执行实例
	Update(ctx context.Context, execute *WorkflowExecute) error

	// Delete 删除工作流执行实例（软删除）
	Delete(ctx context.Context, id uuid.UUID) error

	// FindByID 根据ID查询工作流执行实例
	FindByID(ctx context.Context, id uuid.UUID) (*WorkflowExecute, error)

	// List 查询工作流执行列表
	// 支持过滤条件：team_id、workflow_id、status、trigger_type
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*WorkflowExecute, error)

	// Count 统计工作流执行数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// ListByWorkflowID 根据WorkflowID查询执行列表
	// 用于Workflow详情页的执行历史Tab
	ListByWorkflowID(ctx context.Context, workflowID uuid.UUID, limit, offset int) ([]*WorkflowExecute, error)

	// CountByWorkflowID 统计Workflow的执行次数
	CountByWorkflowID(ctx context.Context, workflowID uuid.UUID) (int64, error)
}

// WorkflowExecuteService 工作流执行服务接口
//
// 定义了工作流执行的所有业务逻辑操作
type WorkflowExecuteService interface {
	// Execute 触发工作流执行 ⭐（核心方法）
	// 创建 WorkflowExecute 实例，批量创建所有 Task，激活第一个 Task
	Execute(ctx context.Context, req *ExecuteRequest) (*WorkflowExecute, error)

	// HandleTaskComplete 处理任务完成 ⭐（核心方法）
	// Task 完成后调用，负责状态流转、参数传递、环境锁定、激活下一个 Task
	HandleTaskComplete(ctx context.Context, taskID uuid.UUID) error

	// FindByID 根据ID查询工作流执行实例
	FindByID(ctx context.Context, id string) (*WorkflowExecute, error)

	// List 查询工作流执行列表
	List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*WorkflowExecute, error)

	// Count 统计工作流执行数量
	Count(ctx context.Context, actions ...filters.Filter) (int64, error)

	// ListByWorkflowID 根据WorkflowID查询执行列表
	ListByWorkflowID(ctx context.Context, workflowID string, limit, offset int) ([]*WorkflowExecute, error)

	// CountByWorkflowID 统计Workflow的执行次数
	CountByWorkflowID(ctx context.Context, workflowID string) (int64, error)

	// Cancel 取消工作流执行
	Cancel(ctx context.Context, id string, userID *uuid.UUID, username string) error

	// Delete 删除工作流执行实例
	Delete(ctx context.Context, id string) error

	// GetTasksByExecuteID 🔥 根据执行实例ID获取任务列表
	// 用于前端详情页显示任务列表
	GetTasksByExecuteID(ctx context.Context, executeID string) ([]*Task, error)
}

// ExecuteRequest 触发执行请求
type ExecuteRequest struct {
	WorkflowID       uuid.UUID              // 工作流ID
	TriggerType      string                 // 触发类型：manual/api/webhook
	UserID           *uuid.UUID             // 触发者用户ID
	Username         string                 // 触发者用户名
	InitialVariables map[string]interface{} // ⭐ 初始变量（用于参数传递）
	MetadataOverride map[string]interface{} // ⭐ Metadata 覆盖（高级用例）
}
