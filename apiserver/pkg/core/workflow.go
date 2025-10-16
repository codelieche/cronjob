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

// Workflow 工作流模板实体
//
// 定义了一个工作流模板的所有属性，包括：
// - 基本信息：名称、Code、描述、项目归属等
// - 步骤信息：编排的步骤列表（JSON格式）
// - 元数据信息：执行环境、Worker配置等
// - 统计信息：执行次数、成功/失败次数等
//
// Workflow 是一组 Task 的模板，定义了任务的执行顺序和初始参数
// 每次执行 Workflow 会创建一个 WorkflowExecute 实例
type Workflow struct {
	ID               uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                                   // 工作流唯一标识
	TeamID           *uuid.UUID      `gorm:"size:256;index:idx_workflow_team_code,priority:1" json:"team_id"` // 团队ID，用于多租户隔离（联合唯一索引：team_id+code）
	Project          string          `gorm:"size:128;index;default:default" json:"project"`                   // 所属项目，用于工作流分组管理
	Code             string          `gorm:"size:128;index:idx_workflow_team_code,priority:2" json:"code"`    // 工作流代码（英文），用于URL路由和快捷访问（联合唯一索引：team_id+code）
	Name             string          `gorm:"size:256" json:"name"`                                            // 工作流名称（友好名称）
	Description      string          `gorm:"size:512" json:"description"`                                     // 工作流描述
	Steps            json.RawMessage `gorm:"type:json" json:"steps" swaggertype:"array,object"`               // 步骤列表（JSON数组），定义工作流的执行步骤
	DefaultVariables json.RawMessage `gorm:"type:json" json:"default_variables" swaggertype:"object"`         // 默认变量（JSON对象），执行时的默认参数值，可被 initial_variables 覆盖
	Metadata         json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`                  // 元数据配置，存储执行环境、Worker配置等
	IsActive         *bool           `gorm:"type:boolean;default:true" json:"is_active"`                      // 是否激活，用于控制是否可以执行
	Timeout          int             `gorm:"type:int;default:0" json:"timeout"`                           // 工作流整体超时时间（秒），0表示使用默认值（24小时）

	// 统计信息（冗余字段，提升查询性能）
	ExecuteCount  int        `gorm:"type:int;default:0" json:"execute_count"`       // 执行次数
	SuccessCount  int        `gorm:"type:int;default:0" json:"success_count"`       // 成功次数
	FailedCount   int        `gorm:"type:int;default:0" json:"failed_count"`        // 失败次数
	LastExecuteAt *time.Time `gorm:"column:last_execute_at" json:"last_execute_at"` // 最后执行时间
	LastStatus    string     `gorm:"size:40" json:"last_status"`                    // 最后执行状态

	// 时间戳字段
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
	Deleted   *bool          `gorm:"type:boolean;default:false" json:"deleted"`          // 软删除标记
}

// TableName 返回数据库表名
func (Workflow) TableName() string {
	return "workflows"
}

// WorkflowStep 工作流步骤定义
//
// 定义了工作流中的单个步骤，包括：
// - 基本信息：名称、描述、执行顺序
// - 执行信息：Category（Runner类型）、Args（参数）
// - 超时配置：Timeout
type WorkflowStep struct {
	Order       int                    `json:"order"`                 // 步骤顺序（从1开始）
	Name        string                 `json:"name"`                  // 步骤名称
	Description string                 `json:"description,omitempty"` // 步骤描述（可选）
	Category    string                 `json:"category"`              // 任务分类（对应Task的Category，如：git/script/container）
	Args        map[string]interface{} `json:"args"`                  // 任务参数（JSON对象，支持 ${variable} 模板替换）
	Timeout     int                    `json:"timeout"`               // 超时时间（秒），0表示不限制
}

// GetSteps 获取解析后的步骤列表
//
// 将JSON格式的Steps字段解析为WorkflowStep数组
//
// 返回：
//   - 解析后的步骤列表
//   - 解析错误（如果有）
func (w *Workflow) GetSteps() ([]WorkflowStep, error) {
	if len(w.Steps) == 0 {
		return []WorkflowStep{}, nil
	}

	var steps []WorkflowStep
	if err := json.Unmarshal(w.Steps, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// SetSteps 设置步骤列表
//
// 将WorkflowStep数组序列化为JSON并存储到Steps字段
//
// 参数：
//   - steps: 步骤列表
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetSteps(steps []WorkflowStep) error {
	data, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	w.Steps = data
	return nil
}

// GetDefaultVariables 获取解析后的默认变量
//
// 将JSON格式的DefaultVariables字段解析为map
//
// 返回：
//   - 解析后的默认变量
//   - 解析错误（如果有）
func (w *Workflow) GetDefaultVariables() (map[string]interface{}, error) {
	if len(w.DefaultVariables) == 0 {
		return make(map[string]interface{}), nil
	}

	var variables map[string]interface{}
	if err := json.Unmarshal(w.DefaultVariables, &variables); err != nil {
		return nil, err
	}
	return variables, nil
}

// SetDefaultVariables 设置默认变量
//
// 将map序列化为JSON并存储到DefaultVariables字段
//
// 参数：
//   - variables: 默认变量
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetDefaultVariables(variables map[string]interface{}) error {
	if variables == nil {
		variables = make(map[string]interface{})
	}
	data, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	w.DefaultVariables = data
	return nil
}

// GetMetadata 获取解析后的元数据
//
// 将JSON格式的Metadata字段解析为Metadata结构体
// 使用统一的 Metadata 结构
//
// 返回：
//   - 解析后的 Metadata 结构体
//   - 解析错误（如果有）
func (w *Workflow) GetMetadata() (*Metadata, error) {
	return ParseMetadata(w.Metadata)
}

// SetMetadata 设置元数据
//
// 将 Metadata 结构体序列化为JSON并存储到Metadata字段
//
// 参数：
//   - metadata: Metadata 结构体
//
// 返回：
//   - 序列化错误（如果有）
func (w *Workflow) SetMetadata(metadata *Metadata) error {
	data, err := SerializeMetadata(metadata)
	if err != nil {
		return err
	}
	w.Metadata = data
	return nil
}

// BeforeCreate GORM钩子：创建前的处理
func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	// 1. 设置ID
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}

	// 2. 设置默认值
	if w.IsActive == nil {
		trueValue := true
		w.IsActive = &trueValue
	}

	// 3. 设置统计信息初始值
	w.ExecuteCount = 0
	w.SuccessCount = 0
	w.FailedCount = 0

	return nil
}

// BeforeDelete 删除前设置deleted字段为True
func (w *Workflow) BeforeDelete(tx *gorm.DB) error {
	// 设置Deleted字段为true
	trueValue := true
	w.Deleted = &trueValue
	return nil
}

// WorkflowStore 工作流数据存储接口
//
// 定义了工作流的所有数据访问操作
type WorkflowStore interface {
	// Create 创建工作流
	Create(ctx context.Context, workflow *Workflow) error

	// Update 更新工作流
	Update(ctx context.Context, workflow *Workflow) error

	// Delete 删除工作流（软删除）
	Delete(ctx context.Context, id uuid.UUID) error

	// FindByID 根据ID查询工作流
	FindByID(ctx context.Context, id uuid.UUID) (*Workflow, error)

	// FindByCode 根据Code查询工作流（团队内唯一）
	FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*Workflow, error)

	// List 查询工作流列表
	// 支持过滤条件：team_id、project、is_active、search（名称/描述）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*Workflow, error)

	// Count 统计工作流数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// UpdateStats 更新统计信息
	// 在WorkflowExecute完成后调用，更新执行次数和最后执行状态
	UpdateStats(ctx context.Context, id uuid.UUID, status string) error
}

// WorkflowService 工作流服务接口
//
// 定义了工作流的所有业务逻辑操作
type WorkflowService interface {
	// Create 创建工作流
	Create(ctx context.Context, workflow *Workflow) error

	// Update 更新工作流
	Update(ctx context.Context, workflow *Workflow) error

	// Delete 删除工作流
	Delete(ctx context.Context, id string) error

	// FindByID 根据ID查询工作流
	FindByID(ctx context.Context, id string) (*Workflow, error)

	// FindByCode 根据Code查询工作流
	FindByCode(ctx context.Context, teamID uuid.UUID, code string) (*Workflow, error)

	// List 查询工作流列表
	List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*Workflow, error)

	// Count 统计工作流数量
	Count(ctx context.Context, actions ...filters.Filter) (int64, error)

	// ToggleActive 切换激活状态
	ToggleActive(ctx context.Context, id string) (*Workflow, error)

	// GetStatistics 获取工作流统计信息
	GetStatistics(ctx context.Context, id string) (map[string]interface{}, error)
}
