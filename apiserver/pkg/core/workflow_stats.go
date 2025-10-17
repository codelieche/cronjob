// Package core 核心数据模型和接口定义
package core

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowStatsDaily Workflow每日统计数据
//
// 存储每个Workflow的每日执行统计，用于提升统计分析查询性能
// 通过后台定时任务每日凌晨自动聚合前一天的数据
//
// 设计要点：
// - WorkflowID + TeamID + StatDate: 唯一标识一个Workflow在某天的统计
// - TeamID: 支持多租户数据隔离
// - 触发方式统计：manual/api/webhook/schedule
// - 步骤统计：平均步骤数、成功步骤数、失败步骤数
type WorkflowStatsDaily struct {
	ID           uuid.UUID  `gorm:"size:36;primaryKey" json:"id"`                                                                   // 统计记录唯一标识
	WorkflowID   uuid.UUID  `gorm:"size:36;uniqueIndex:idx_workflow_team_date;not null;index:idx_workflow_date" json:"workflow_id"` // 工作流ID
	WorkflowName string     `gorm:"size:256;comment:工作流名称" json:"workflow_name"`                                                    // 工作流名称（冗余字段，方便查询展示）
	TeamID       *uuid.UUID `gorm:"size:36;uniqueIndex:idx_workflow_team_date;index:idx_team_date" json:"team_id"`                  // 团队ID，用于多租户隔离

	// 统计日期
	StatDate time.Time `gorm:"type:date;uniqueIndex:idx_workflow_team_date;index:idx_stat_date;index:idx_workflow_date,priority:2" json:"stat_date"` // 统计日期（与workflow_id/team_id组成唯一约束）

	// 🔥 执行数量统计
	TotalExecutes    int `gorm:"type:int;default:0;comment:总执行次数" json:"total_executes"`     // 总执行次数
	SuccessExecutes  int `gorm:"type:int;default:0;comment:成功执行次数" json:"success_executes"`  // 成功执行次数
	FailedExecutes   int `gorm:"type:int;default:0;comment:失败执行次数" json:"failed_executes"`   // 失败执行次数
	CanceledExecutes int `gorm:"type:int;default:0;comment:取消执行次数" json:"canceled_executes"` // 取消执行次数

	// 🔥 执行效率统计（单位：秒）
	AvgDuration float64 `gorm:"type:decimal(10,2);default:0;comment:平均执行时长(秒)" json:"avg_duration"` // 平均执行时长
	MinDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最小执行时长(秒)" json:"min_duration"` // 最小执行时长
	MaxDuration float64 `gorm:"type:decimal(10,2);default:0;comment:最大执行时长(秒)" json:"max_duration"` // 最大执行时长

	// 🔥 步骤统计
	AvgTotalSteps   float64 `gorm:"type:decimal(10,2);default:0;comment:平均总步骤数" json:"avg_total_steps"`    // 平均总步骤数
	AvgSuccessSteps float64 `gorm:"type:decimal(10,2);default:0;comment:平均成功步骤数" json:"avg_success_steps"` // 平均成功步骤数
	AvgFailedSteps  float64 `gorm:"type:decimal(10,2);default:0;comment:平均失败步骤数" json:"avg_failed_steps"`  // 平均失败步骤数

	// 🔥 触发方式统计
	ManualTriggers   int `gorm:"type:int;default:0;comment:手动触发次数" json:"manual_triggers"`       // 手动触发次数
	ApiTriggers      int `gorm:"type:int;default:0;comment:API触发次数" json:"api_triggers"`         // API触发次数
	WebhookTriggers  int `gorm:"type:int;default:0;comment:Webhook触发次数" json:"webhook_triggers"` // Webhook触发次数
	ScheduleTriggers int `gorm:"type:int;default:0;comment:定时触发次数" json:"schedule_triggers"`     // 定时触发次数（保留字段）

	// 时间戳
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"` // 创建时间
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"` // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间
}

// TableName 表名
func (WorkflowStatsDaily) TableName() string {
	return "workflow_stats_daily"
}

// BeforeCreate GORM钩子：创建前生成UUID
func (m *WorkflowStatsDaily) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// WorkflowStatsStore Workflow统计数据存储接口
//
// 定义了Workflow统计数据的所有数据访问操作
type WorkflowStatsStore interface {
	// Create 创建统计记录
	Create(ctx context.Context, stats *WorkflowStatsDaily) error

	// Update 更新统计记录
	Update(ctx context.Context, stats *WorkflowStatsDaily) error

	// FindByWorkflowAndDate 根据WorkflowID和日期查询统计
	FindByWorkflowAndDate(ctx context.Context, workflowID uuid.UUID, teamID *uuid.UUID, date time.Time) (*WorkflowStatsDaily, error)

	// GetDailyStats 获取最近N天的统计数据（按日期聚合）
	// teamID: 团队ID（为空则查询所有团队）
	// days: 天数
	GetDailyStats(ctx context.Context, teamID *uuid.UUID, days int) ([]WorkflowStatsDaily, error)

	// GetWorkflowRanking 获取Workflow执行排行（按总执行次数）
	// 用于展示Top N Workflows
	GetWorkflowRanking(ctx context.Context, teamID *uuid.UUID, days int, limit int) ([]map[string]interface{}, error)

	// GetByDateRange 获取指定日期范围的统计数据
	GetByDateRange(ctx context.Context, teamID *uuid.UUID, startDate, endDate time.Time) ([]WorkflowStatsDaily, error)
}

// WorkflowStatsService Workflow统计服务接口
//
// 定义了Workflow统计的所有业务逻辑操作
type WorkflowStatsService interface {
	// AggregateDailyStats 聚合指定日期的统计数据
	// 从 workflow_executes 表聚合到 workflow_stats_daily 表
	AggregateDailyStats(ctx context.Context, date time.Time) error

	// AggregateHistoricalStats 聚合历史统计数据（批量）
	// 用于初次部署或补充历史数据
	AggregateHistoricalStats(ctx context.Context, startDate, endDate time.Time) error

	// GetSuccessRateTrend 获取成功率趋势
	// 返回最近N天每天的执行统计（total, success, failed, success_rate）
	GetSuccessRateTrend(ctx context.Context, teamID *uuid.UUID, days int) (map[string]interface{}, error)

	// GetExecutionEfficiency 获取执行效率统计
	// 返回平均时长、执行次数等
	GetExecutionEfficiency(ctx context.Context, teamID *uuid.UUID, days int) (map[string]interface{}, error)

	// GetWorkflowRanking 获取Workflow排行榜
	// 返回执行次数最多的Top N Workflows
	GetWorkflowRanking(ctx context.Context, teamID *uuid.UUID, days int) (map[string]interface{}, error)

	// GetTimeDistribution 获取时间分布统计
	// 返回按星期几的执行分布
	GetTimeDistribution(ctx context.Context, teamID *uuid.UUID, days int) (map[string]interface{}, error)

	// GetPeriodComparison 获取时间段对比
	// 返回本周vs上周、本月vs上月的对比数据
	GetPeriodComparison(ctx context.Context, teamID *uuid.UUID) (map[string]interface{}, error)
}
