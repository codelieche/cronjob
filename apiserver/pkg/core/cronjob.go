// Package core 核心数据模型和接口定义
// 
// 包含系统中所有核心业务实体的数据模型定义
// 以及相关的数据访问接口和服务接口
package core

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/google/uuid"
)

// CronJob 定时任务实体
// 
// 定义了一个定时任务的所有属性，包括：
// - 基本信息：名称、描述、项目归属等
// - 调度信息：cron表达式、激活状态等
// - 执行信息：命令、参数、超时设置等
// - 状态信息：上次计划时间、执行时间、执行状态等
//
// 这是系统的核心实体，用于定义何时执行什么任务
type CronJob struct {
	types.BaseModel
	ID           uuid.UUID  `gorm:"size:256;primaryKey" json:"id"`                 // 定时任务唯一标识
	Project      string     `gorm:"size:128;index;default:default" json:"project"` // 所属项目，用于任务分组管理，默认为"default"
	Category     string     `gorm:"size:128;index;not null" json:"category"`       // 任务分类编码，用于任务类型管理，不能为空
	Name         string     `gorm:"size:128" json:"name"`                          // 任务名称，便于识别和管理
	Time         string     `gorm:"size:100" json:"time"`                          // cron时间表达式，定义任务执行时间规则
	Command      string     `gorm:"size:512" json:"command"`                       // 要执行的命令，支持系统命令和脚本
	Args         string     `gorm:"size:512" json:"args"`                          // 命令参数，JSON格式存储
	Description  string     `gorm:"size:512" json:"description"`                   // 任务描述，说明任务用途和注意事项
	LastPlan     *time.Time `gorm:"column:last_plan" json:"last_plan"`             // 上次计划执行时间，用于调度计算
	LastDispatch *time.Time `gorm:"column:last_dispatch" json:"last_dispatch"`     // 上次实际执行时间，用于监控和统计
	LastStatus   string     `gorm:"size:128" json:"last_status"`                   // 上次执行状态，用于监控任务健康度
	IsActive     bool       `gorm:"type:boolean;default:false" json:"is_active"`   // 是否激活，只有激活的任务才会被调度执行
	SaveLog      bool       `gorm:"type:boolean;default:true" json:"save_log"`     // 是否保存执行日志，用于调试和审计
	Timeout      int        `gorm:"type:int;default:0" json:"timeout"`             // 任务超时时间（秒），0表示不限制
}

// TableName 返回数据库表名
// 实现GORM的TableName接口，指定CronJob对应的数据库表名
func (CronJob) TableName() string {
	return "cronjobs"
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
