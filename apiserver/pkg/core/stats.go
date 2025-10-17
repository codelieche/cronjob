// Package core 核心数据模型和接口定义
package core

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskStatsDaily 任务每日统计数据
//
// 存储每日的任务执行统计汇总数据，用于提升统计分析查询性能
// 通过后台定时任务每日凌晨自动聚合前一天的数据
//
// 设计要点：
// - TeamID: 支持多租户数据隔离
// - StatDate: 统计日期，与TeamID组成复合索引
// - 任务数量统计：总数、成功、失败、错误、超时
// - 执行效率统计：平均/最小/最大执行时长
// - 队列健康度：最大/平均pending数量
type TaskStatsDaily struct {
	ID     uuid.UUID  `gorm:"size:36;primaryKey" json:"id"`                            // 统计记录唯一标识
	TeamID *uuid.UUID `gorm:"size:36;uniqueIndex:idx_team_date_unique" json:"team_id"` // 团队ID，用于多租户隔离

	// 统计日期
	StatDate time.Time `gorm:"type:date;uniqueIndex:idx_team_date_unique;index:idx_stat_date" json:"stat_date"` // 统计日期（与team_id组成唯一约束）

	// 🔥 任务数量统计
	TotalTasks   int `gorm:"type:int;default:0;comment:总任务数" json:"total_tasks"`    // 总任务数
	SuccessTasks int `gorm:"type:int;default:0;comment:成功任务数" json:"success_tasks"` // 成功任务数
	FailedTasks  int `gorm:"type:int;default:0;comment:失败任务数" json:"failed_tasks"`  // 失败任务数（failed状态）
	ErrorTasks   int `gorm:"type:int;default:0;comment:错误任务数" json:"error_tasks"`   // 错误任务数（error状态）
	TimeoutTasks int `gorm:"type:int;default:0;comment:超时任务数" json:"timeout_tasks"` // 超时任务数（timeout状态）

	// 🔥 执行效率统计（单位：秒）
	AvgDuration float64 `gorm:"type:decimal(10,2);default:0;comment:平均执行时长(秒)" json:"avg_duration"` // 平均执行时长
	MinDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最小执行时长(秒)" json:"min_duration"` // 最小执行时长
	MaxDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最大执行时长(秒)" json:"max_duration"` // 最大执行时长

	// 🔥 队列健康度统计
	MaxPendingCount int `gorm:"type:int;default:0;comment:最大pending任务数" json:"max_pending_count"` // 当天最大pending任务数
	AvgPendingCount int `gorm:"type:int;default:0;comment:平均pending任务数" json:"avg_pending_count"` // 当天平均pending任务数

	// 时间戳
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
}

// TableName 表名
func (TaskStatsDaily) TableName() string {
	return "task_stats_daily"
}

// BeforeCreate GORM钩子：创建前生成UUID
func (m *TaskStatsDaily) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// CronjobStatsDaily 定时任务每日统计数据
//
// 存储每个CronJob的每日执行统计，用于对比各定时任务的执行情况
//
// 设计要点：
// - CronjobID + StatDate: 唯一标识一个CronJob在某天的统计
// - TeamID: 支持多租户数据隔离
// - 与TaskStatsDaily类似的统计维度，但细化到具体的CronJob
type CronjobStatsDaily struct {
	ID          uuid.UUID  `gorm:"size:36;primaryKey" json:"id"`                                                        // 统计记录唯一标识
	CronjobID   uuid.UUID  `gorm:"size:36;uniqueIndex:idx_cronjob_team_date_unique;not null" json:"cronjob_id"`         // 定时任务ID
	CronjobName string     `gorm:"size:256;comment:定时任务名称" json:"cronjob_name"`                                         // 定时任务名称（冗余字段，方便查询展示）
	TeamID      *uuid.UUID `gorm:"size:36;uniqueIndex:idx_cronjob_team_date_unique;index:idx_team_date" json:"team_id"` // 团队ID，用于多租户隔离

	// 统计日期
	StatDate time.Time `gorm:"type:date;uniqueIndex:idx_cronjob_team_date_unique;index:idx_stat_date" json:"stat_date"` // 统计日期（与cronjob_id/team_id组成唯一约束）

	// 🔥 任务数量统计
	TotalTasks   int `gorm:"type:int;default:0;comment:总任务数" json:"total_tasks"`    // 总任务数
	SuccessTasks int `gorm:"type:int;default:0;comment:成功任务数" json:"success_tasks"` // 成功任务数
	FailedTasks  int `gorm:"type:int;default:0;comment:失败任务数" json:"failed_tasks"`  // 失败任务数
	ErrorTasks   int `gorm:"type:int;default:0;comment:错误任务数" json:"error_tasks"`   // 错误任务数
	TimeoutTasks int `gorm:"type:int;default:0;comment:超时任务数" json:"timeout_tasks"` // 超时任务数

	// 🔥 执行效率统计（单位：秒）
	AvgDuration float64 `gorm:"type:decimal(10,2);default:0;comment:平均执行时长(秒)" json:"avg_duration"` // 平均执行时长
	MinDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最小执行时长(秒)" json:"min_duration"` // 最小执行时长
	MaxDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最大执行时长(秒)" json:"max_duration"` // 最大执行时长

	// 时间戳
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
}

// TableName 表名
func (CronjobStatsDaily) TableName() string {
	return "cronjob_stats_daily"
}

// BeforeCreate GORM钩子：创建前生成UUID
func (m *CronjobStatsDaily) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// WorkerStatsDaily Worker节点每日统计数据
//
// 存储每个Worker节点的每日执行统计，用于监控各Worker的负载和健康状态
//
// 设计要点：
// - WorkerID + StatDate: 唯一标识一个Worker在某天的统计
// - TeamID: 支持多租户数据隔离（Worker可能跨团队）
// - 用于识别高负载Worker、故障Worker、负载均衡优化
type WorkerStatsDaily struct {
	ID       uuid.UUID  `gorm:"size:36;primaryKey" json:"id"`                                                       // 统计记录唯一标识
	WorkerID uuid.UUID  `gorm:"size:36;uniqueIndex:idx_worker_team_date_unique;not null" json:"worker_id"`          // Worker节点ID
	TeamID   *uuid.UUID `gorm:"size:36;uniqueIndex:idx_worker_team_date_unique;index:idx_team_date" json:"team_id"` // 团队ID，用于多租户隔离

	// Worker信息
	WorkerName string `gorm:"size:256;comment:Worker名称" json:"worker_name"` // Worker名称（冗余字段，方便查询）

	// 统计日期
	StatDate time.Time `gorm:"type:date;uniqueIndex:idx_worker_team_date_unique;index:idx_stat_date" json:"stat_date"` // 统计日期（与worker_id/team_id组成唯一约束）

	// 🔥 任务数量统计
	TotalTasks   int `gorm:"type:int;default:0;comment:总任务数" json:"total_tasks"`    // 总任务数
	SuccessTasks int `gorm:"type:int;default:0;comment:成功任务数" json:"success_tasks"` // 成功任务数
	FailedTasks  int `gorm:"type:int;default:0;comment:失败任务数" json:"failed_tasks"`  // 失败任务数
	ErrorTasks   int `gorm:"type:int;default:0;comment:错误任务数" json:"error_tasks"`   // 错误任务数
	TimeoutTasks int `gorm:"type:int;default:0;comment:超时任务数" json:"timeout_tasks"` // 超时任务数

	// 🔥 执行效率统计（单位：秒）
	AvgDuration float64 `gorm:"type:decimal(10,2);default:0;comment:平均执行时长(秒)" json:"avg_duration"` // 平均执行时长
	MinDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最小执行时长(秒)" json:"min_duration"` // 最小执行时长
	MaxDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最大执行时长(秒)" json:"max_duration"` // 最大执行时长

	// 时间戳
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
}

// TableName 表名
func (WorkerStatsDaily) TableName() string {
	return "worker_stats_daily"
}

// BeforeCreate GORM钩子：创建前生成UUID
func (m *WorkerStatsDaily) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
