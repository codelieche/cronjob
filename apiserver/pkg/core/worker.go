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

// WorkerMetadata 工作节点元数据
//
// 存储工作节点的能力信息，如支持的任务类型、配置等
type WorkerMetadata struct {
	Tasks []string `json:"tasks"` // 支持处理的tasks类型
}

// Worker 工作节点实体
//
// 代表系统中执行具体任务的节点，包括：
// - 基本信息：节点名称、描述、状态等
// - 能力信息：支持的任务类型、元数据等
// - 状态信息：活跃状态、最后活跃时间等
//
// Worker节点通过心跳机制保持与API Server的连接
// 并通过WebSocket接收任务执行指令
type Worker struct {
	ID          uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                                // Worker节点唯一标识
	Name        string          `gorm:"size:80;index" json:"name"`                                    // Worker节点名称，用于识别
	Description string          `gorm:"text" json:"description"`                                      // Worker节点描述信息
	IsActive    *bool           `gorm:"type:boolean;default:false" json:"is_active" form:"is_active"` // 是否活跃，只有活跃的节点才会接收任务
	LastActive  *time.Time      `gorm:"column:last_active" json:"last_active"`                        // 最后活跃时间，用于健康检查
	Metadata    json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`               // 元数据，存储节点能力、配置等信息
	CreatedAt   time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`           // 节点注册时间
	UpdatedAt   time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`           // 节点最后更新时间
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`                                               // 软删除时间
	Deleted     *bool           `gorm:"type:boolean;default:false" json:"deleted" form:"deleted"`     // 软删除标记
}

// TableName 工作节点表名
func (Worker) TableName() string {
	return "workers"
}

// WorkerStore 工作节点存储接口
type WorkerStore interface {
	// FindByID 根据ID获取工作节点
	FindByID(ctx context.Context, id uuid.UUID) (*Worker, error)

	// FindByName 根据名称获取工作节点
	FindByName(ctx context.Context, name string) (*Worker, error)

	// Create 创建工作节点
	Create(ctx context.Context, obj *Worker) (*Worker, error)

	// Update 更新工作节点信息
	Update(ctx context.Context, obj *Worker) (*Worker, error)

	// Delete 删除工作节点
	Delete(ctx context.Context, obj *Worker) error

	// DeleteByID 根据ID删除工作节点
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// List 获取工作节点列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (workers []*Worker, err error)

	// Count 统计工作节点数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建工作节点
	GetOrCreate(ctx context.Context, obj *Worker) (*Worker, error)
}

// WorkerService 工作节点服务接口
type WorkerService interface {
	// FindByID 根据ID获取工作节点
	FindByID(ctx context.Context, id string) (*Worker, error)

	// FindByName 根据名称获取工作节点
	FindByName(ctx context.Context, name string) (*Worker, error)

	// Create 创建工作节点
	Create(ctx context.Context, obj *Worker) (*Worker, error)

	// Update 更新工作节点信息
	Update(ctx context.Context, obj *Worker) (*Worker, error)

	// Delete 删除工作节点
	Delete(ctx context.Context, obj *Worker) error

	// DeleteByID 根据ID删除工作节点
	DeleteByID(ctx context.Context, id string) error

	// List 获取工作节点列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (workers []*Worker, err error)

	// Count 统计工作节点数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetOrCreate 获取或者创建工作节点
	GetOrCreate(ctx context.Context, obj *Worker) (*Worker, error)

	// CheckAndUpdateInactiveWorkers 检查并更新失活的worker
	CheckAndUpdateInactiveWorkers(ctx context.Context, inactiveDuration time.Duration) (int, error)

	// CheckWorkerStatusLoop 循环检查worker状态的后台任务
	CheckWorkerStatusLoop(ctx context.Context, checkInterval time.Duration, inactiveDuration time.Duration)
}
