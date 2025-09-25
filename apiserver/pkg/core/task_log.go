package core

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 任务日志存储类型常量
const (
	TaskLogStorageDB   = "db"   // 数据库存储
	TaskLogStorageFile = "file" // 文件存储
	TaskLogStorageS3   = "s3"   // S3/MinIO存储
)

// TaskLog 任务日志实体
//
// 记录任务执行的详细日志信息，支持多种存储方式：
// - db: 存储在数据库的content字段中
// - file: 存储在文件系统中，路径为 ./logs/task/:task_id.log
// - s3: 存储在MinIO/S3对象存储中
//
// 通过Storage字段区分存储类型，Path字段记录具体的存储路径
type TaskLog struct {
	TaskID    uuid.UUID      `gorm:"primaryKey;size:256;not null" json:"task_id"`        // 主键：任务ID
	Storage   string         `gorm:"size:20;default:'db';not null" json:"storage"`       // 存储类型：db/file/s3
	Path      string         `gorm:"size:512;index:idx_path" json:"path"`                // 存储路径（文件路径或S3对象键）
	Content   string         `gorm:"column:content;type:longtext" json:"content"`        // 日志内容（仅db存储时使用）
	Size      int64          `gorm:"type:bigint;default:0" json:"size"`                  // 日志大小（字节）
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 最后更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
	Deleted   *bool          `gorm:"type:boolean;default:false" json:"deleted"`          // 软删除标记
}

// TableName 任务日志表名
func (TaskLog) TableName() string {
	return "task_logs"
}

// TaskLogStore 任务日志存储接口
type TaskLogStore interface {
	// FindByTaskID 根据任务ID获取任务日志
	FindByTaskID(ctx context.Context, taskID uuid.UUID) (*TaskLog, error)

	// Create 创建任务日志
	Create(ctx context.Context, obj *TaskLog) (*TaskLog, error)

	// Update 更新任务日志信息
	Update(ctx context.Context, obj *TaskLog) (*TaskLog, error)

	// Delete 删除任务日志
	Delete(ctx context.Context, obj *TaskLog) error

	// DeleteByTaskID 根据任务ID删除任务日志
	DeleteByTaskID(ctx context.Context, taskID uuid.UUID) error

	// List 获取任务日志列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (logs []*TaskLog, err error)

	// Count 统计任务日志数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)
}

// TaskLogService 任务日志服务接口
type TaskLogService interface {
	// FindByTaskID 根据任务ID获取任务日志
	FindByTaskID(ctx context.Context, taskID string) (*TaskLog, error)

	// Create 创建任务日志
	Create(ctx context.Context, obj *TaskLog) (*TaskLog, error)

	// Update 更新任务日志信息
	Update(ctx context.Context, obj *TaskLog) (*TaskLog, error)

	// Delete 删除任务日志
	Delete(ctx context.Context, obj *TaskLog) error

	// DeleteByTaskID 根据任务ID删除任务日志
	DeleteByTaskID(ctx context.Context, taskID string) error

	// List 获取任务日志列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (logs []*TaskLog, err error)

	// Count 统计任务日志数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// GetLogContent 获取日志内容（根据存储类型读取）
	GetLogContent(ctx context.Context, log *TaskLog) (string, error)

	// SaveLogContent 保存日志内容（根据存储类型保存）
	SaveLogContent(ctx context.Context, log *TaskLog, content string) error

	// AppendLogContent 追加日志内容
	AppendLogContent(ctx context.Context, log *TaskLog, content string) (*TaskLog, error)
}
