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
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/google/uuid"
)

// CronJobMetadata 定时任务元数据
//
// 定义定时任务的执行环境和配置信息，包括：
// - 执行环境：工作目录、环境变量等
// - Worker配置：指定执行节点、节点标签等
// - 扩展配置：其他自定义配置信息
type CronJobMetadata struct {
	WorkingDir    string                 `json:"workingDir,omitempty"`     // 任务执行的工作目录
	Environment   map[string]string      `json:"environment,omitempty"`    // 环境变量设置
	WorkerSelect  []string               `json:"worker_select,omitempty"`  // 可执行此任务的Worker节点名称列表，空表示所有Worker
	WorkerLabels  map[string]string      `json:"worker_labels,omitempty"`  // Worker节点标签选择器
	Priority      int                    `json:"priority,omitempty"`       // 任务优先级（1-10，默认5）
	ResourceLimit map[string]string      `json:"resource_limit,omitempty"` // 资源限制配置
	Extensions    map[string]interface{} `json:"extensions,omitempty"`     // 扩展字段，用于存储其他自定义配置
}

// CronJob 定时任务实体
//
// 定义了一个定时任务的所有属性，包括：
// - 基本信息：名称、描述、项目归属等
// - 调度信息：cron表达式、激活状态等
// - 执行信息：命令、参数、超时设置等
// - 状态信息：上次计划时间、执行时间、执行状态等
// - 元数据信息：执行环境、Worker配置等
//
// 这是系统的核心实体，用于定义何时执行什么任务
type CronJob struct {
	types.BaseModel
	ID           uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                 // 定时任务唯一标识
	Project      string          `gorm:"size:128;index;default:default" json:"project"` // 所属项目，用于任务分组管理，默认为"default"
	Category     string          `gorm:"size:128;index;not null" json:"category"`       // 任务分类编码，用于任务类型管理，不能为空
	Name         string          `gorm:"size:128" json:"name"`                          // 任务名称，便于识别和管理
	Time         string          `gorm:"size:100" json:"time"`                          // cron时间表达式，定义任务执行时间规则
	Command      string          `gorm:"size:512" json:"command"`                       // 要执行的命令，支持系统命令和脚本
	Args         string          `gorm:"size:512" json:"args"`                          // 命令参数，JSON格式存储
	Description  string          `gorm:"size:512" json:"description"`                   // 任务描述，说明任务用途和注意事项
	LastPlan     *time.Time      `gorm:"column:last_plan" json:"last_plan"`             // 上次计划执行时间，用于调度计算
	LastDispatch *time.Time      `gorm:"column:last_dispatch" json:"last_dispatch"`     // 上次实际执行时间，用于监控和统计
	LastStatus   string          `gorm:"size:128" json:"last_status"`                   // 上次执行状态，用于监控任务健康度
	IsActive     *bool           `gorm:"type:boolean;default:false" json:"is_active"`   // 是否激活，只有激活的任务才会被调度执行
	SaveLog      *bool           `gorm:"type:boolean;default:true" json:"save_log"`     // 是否保存执行日志，用于调试和审计
	Timeout      int             `gorm:"type:int;default:0" json:"timeout"`             // 任务超时时间（秒），0表示不限制
	Metadata     json.RawMessage `gorm:"type:json" json:"metadata"`                     // 任务元数据，存储执行环境、Worker配置等信息
}

// TableName 返回数据库表名
// 实现GORM的TableName接口，指定CronJob对应的数据库表名
func (CronJob) TableName() string {
	return "cronjobs"
}

// GetMetadata 获取解析后的元数据
// 将JSON格式的Metadata字段解析为CronJobMetadata结构体
func (c *CronJob) GetMetadata() (*CronJobMetadata, error) {
	if len(c.Metadata) == 0 {
		return &CronJobMetadata{}, nil
	}

	var metadata CronJobMetadata
	if err := json.Unmarshal(c.Metadata, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// SetMetadata 设置元数据
// 将CronJobMetadata结构体序列化为JSON并存储到Metadata字段
func (c *CronJob) SetMetadata(metadata *CronJobMetadata) error {
	if metadata == nil {
		c.Metadata = nil
		return nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	c.Metadata = data
	return nil
}

// CronJobStore 定时任务数据存储接口
//
// 定义了定时任务的所有数据访问操作
// 包括基本的CRUD操作和业务相关的查询操作
// 实现了数据访问层与业务逻辑层的解耦
type CronJobStore interface {
	// FindByID 根据ID获取定时任务
	FindByID(ctx context.Context, id uuid.UUID) (*CronJob, error)

	// FindByName 根据名称获取定时任务
	FindByName(ctx context.Context, name string) (*CronJob, error)

	// FindByProjectAndName 根据项目和名称获取定时任务
	FindByProjectAndName(ctx context.Context, project string, name string) (*CronJob, error)

	// Create 创建定时任务
	Create(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Update 更新定时任务信息
	Update(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Delete 删除定时任务
	Delete(ctx context.Context, obj *CronJob) error

	// DeleteByID 根据ID删除定时任务
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取定时任务列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (CronJobs []*CronJob, err error)

	// Count 统计定时任务数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建定时任务
	GetOrCreate(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Patch 动态更新定时任务字段
	Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
}

// CronJobService 定时任务服务接口
type CronJobService interface {
	// FindByID 根据ID获取定时任务
	FindByID(ctx context.Context, id string) (*CronJob, error)

	// FindByName 根据名称获取定时任务
	FindByName(ctx context.Context, name string) (*CronJob, error)

	// FindByProjectAndName 根据名称获取定时任务
	FindByProjectAndName(ctx context.Context, project string, name string) (*CronJob, error)

	// Create 创建定时任务
	Create(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Update 更新定时任务信息
	Update(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Delete 删除定时任务
	Delete(ctx context.Context, obj *CronJob) error

	// DeleteByID 根据ID删除定时任务
	DeleteByID(ctx context.Context, id string) error

	// List 获取定时任务列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (CronJobs []*CronJob, err error)

	// Count 统计定时任务数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建定时任务
	GetOrCreate(ctx context.Context, obj *CronJob) (*CronJob, error)

	// Patch 动态更新定时任务字段
	Patch(ctx context.Context, id string, updates map[string]interface{}) error
}
