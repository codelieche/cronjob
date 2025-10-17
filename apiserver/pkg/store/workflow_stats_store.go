package store

import (
	"context"
	"errors"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowStatsStore Workflow统计数据存储
//
// 实现 core.WorkflowStatsStore 接口
type WorkflowStatsStore struct {
	db *gorm.DB
}

// NewWorkflowStatsStore 创建Store实例
func NewWorkflowStatsStore(db *gorm.DB) *WorkflowStatsStore {
	return &WorkflowStatsStore{db: db}
}

// Create 创建统计记录
func (s *WorkflowStatsStore) Create(ctx context.Context, stats *core.WorkflowStatsDaily) error {
	return s.db.WithContext(ctx).Create(stats).Error
}

// Update 更新统计记录
func (s *WorkflowStatsStore) Update(ctx context.Context, stats *core.WorkflowStatsDaily) error {
	return s.db.WithContext(ctx).Save(stats).Error
}

// FindByWorkflowAndDate 根据WorkflowID和日期查询统计
func (s *WorkflowStatsStore) FindByWorkflowAndDate(
	ctx context.Context,
	workflowID uuid.UUID,
	teamID *uuid.UUID,
	date time.Time,
) (*core.WorkflowStatsDaily, error) {
	var stats core.WorkflowStatsDaily

	query := s.db.WithContext(ctx).
		Where("workflow_id = ? AND stat_date = ?", workflowID, date.Format("2006-01-02"))

	// 如果指定了团队ID，添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	err := query.First(&stats).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	return &stats, nil
}

// GetDailyStats 获取最近N天的统计数据
// teamID: 团队ID（为空则查询所有团队）
// days: 天数
func (s *WorkflowStatsStore) GetDailyStats(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
) ([]core.WorkflowStatsDaily, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query := s.db.WithContext(ctx).
		Where("stat_date >= ?", startDate).
		Order("stat_date ASC")

	// 如果指定了团队ID，添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	var stats []core.WorkflowStatsDaily
	err := query.Find(&stats).Error
	return stats, err
}

// GetWorkflowRanking 获取Workflow执行排行（按总执行次数）
// 用于展示Top N Workflows
func (s *WorkflowStatsStore) GetWorkflowRanking(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
	limit int,
) ([]map[string]interface{}, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query := s.db.WithContext(ctx).
		Table("workflow_stats_daily").
		Select(`
			workflow_id,
			workflow_name,
			SUM(total_executes) as total,
			SUM(success_executes) as success,
			SUM(failed_executes) as failed,
			SUM(canceled_executes) as canceled,
			AVG(avg_duration) as avg_duration,
			ROUND(SUM(success_executes) * 100.0 / NULLIF(SUM(total_executes), 0), 1) as success_rate
		`).
		Where("stat_date >= ? AND deleted_at IS NULL", startDate).
		Group("workflow_id, workflow_name").
		Order("total DESC").
		Limit(limit)

	// 如果指定了团队ID，添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	var results []map[string]interface{}
	err := query.Scan(&results).Error
	return results, err
}

// GetByDateRange 获取指定日期范围的统计数据
func (s *WorkflowStatsStore) GetByDateRange(
	ctx context.Context,
	teamID *uuid.UUID,
	startDate, endDate time.Time,
) ([]core.WorkflowStatsDaily, error) {
	query := s.db.WithContext(ctx).
		Where("stat_date >= ? AND stat_date <= ?",
			startDate.Format("2006-01-02"),
			endDate.Format("2006-01-02")).
		Order("stat_date ASC")

	// 如果指定了团队ID，添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	var stats []core.WorkflowStatsDaily
	err := query.Find(&stats).Error
	return stats, err
}

// GetAggregateStats 获取聚合统计数据（用于时间段对比）
// 返回指定时间范围内的总计数据
func (s *WorkflowStatsStore) GetAggregateStats(
	ctx context.Context,
	teamID *uuid.UUID,
	startDate, endDate time.Time,
) (map[string]interface{}, error) {
	query := s.db.WithContext(ctx).
		Table("workflow_stats_daily").
		Select(`
			SUM(total_executes) as total,
			SUM(success_executes) as success,
			SUM(failed_executes) as failed,
			SUM(canceled_executes) as canceled,
			AVG(avg_duration) as avg_duration,
			ROUND(SUM(success_executes) * 100.0 / NULLIF(SUM(total_executes), 0), 1) as success_rate
		`).
		Where("stat_date >= ? AND stat_date <= ? AND deleted_at IS NULL",
			startDate.Format("2006-01-02"),
			endDate.Format("2006-01-02"))

	// 如果指定了团队ID，添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	var result map[string]interface{}
	err := query.Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}
