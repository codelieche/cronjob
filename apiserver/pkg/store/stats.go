package store

import (
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StatsStore 统计数据存储层
//
// 负责统计汇总表的数据访问操作
type StatsStore struct {
	db *gorm.DB
}

// NewStatsStore 创建统计数据存储实例
func NewStatsStore(db *gorm.DB) *StatsStore {
	return &StatsStore{db: db}
}

// GetTaskStatsDailyByDateRange 查询指定日期范围的任务统计
//
// 参数:
//   - teamID: 团队ID（可选，nil表示不过滤）
//   - startDate: 开始日期
//   - endDate: 结束日期（可选）
//
// 返回值:
//   - []core.TaskStatsDaily: 统计数据列表
//   - error: 错误信息
func (s *StatsStore) GetTaskStatsDailyByDateRange(teamID *uuid.UUID, startDate time.Time, endDate *time.Time) ([]core.TaskStatsDaily, error) {
	var stats []core.TaskStatsDaily

	query := s.db.Where("stat_date >= ?", startDate.Format("2006-01-02")).
		Order("stat_date ASC")

	// 添加团队过滤
	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	// 添加结束日期过滤
	if endDate != nil {
		query = query.Where("stat_date <= ?", endDate.Format("2006-01-02"))
	}

	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// GetCronjobStatsDailyByDateRange 查询指定日期范围的CronJob统计
//
// 参数:
//   - teamID: 团队ID（可选）
//   - startDate: 开始日期
//   - endDate: 结束日期（可选）
//
// 返回值:
//   - []core.CronjobStatsDaily: 统计数据列表
//   - error: 错误信息
func (s *StatsStore) GetCronjobStatsDailyByDateRange(teamID *uuid.UUID, startDate time.Time, endDate *time.Time) ([]core.CronjobStatsDaily, error) {
	var stats []core.CronjobStatsDaily

	query := s.db.Where("stat_date >= ?", startDate.Format("2006-01-02")).
		Order("stat_date ASC")

	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	if endDate != nil {
		query = query.Where("stat_date <= ?", endDate.Format("2006-01-02"))
	}

	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// GetWorkerStatsDailyByDateRange 查询指定日期范围的Worker统计
//
// 参数:
//   - teamID: 团队ID（可选）
//   - startDate: 开始日期
//   - endDate: 结束日期（可选）
//
// 返回值:
//   - []core.WorkerStatsDaily: 统计数据列表
//   - error: 错误信息
func (s *StatsStore) GetWorkerStatsDailyByDateRange(teamID *uuid.UUID, startDate time.Time, endDate *time.Time) ([]core.WorkerStatsDaily, error) {
	var stats []core.WorkerStatsDaily

	query := s.db.Where("stat_date >= ?", startDate.Format("2006-01-02")).
		Order("stat_date ASC")

	if teamID != nil {
		query = query.Where("team_id = ?", teamID)
	}

	if endDate != nil {
		query = query.Where("stat_date <= ?", endDate.Format("2006-01-02"))
	}

	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// GetCronjobStatsDailyGrouped 按CronJob分组查询统计数据
//
// 参数:
//   - teamID: 团队ID（可选）
//   - startDate: 开始日期
//   - endDate: 结束日期（可选）
//
// 返回值:
//   - map[uuid.UUID][]core.CronjobStatsDaily: 按cronjob_id分组的统计数据
//   - error: 错误信息
func (s *StatsStore) GetCronjobStatsDailyGrouped(teamID *uuid.UUID, startDate time.Time, endDate *time.Time) (map[uuid.UUID][]core.CronjobStatsDaily, error) {
	stats, err := s.GetCronjobStatsDailyByDateRange(teamID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 按 cronjob_id 分组
	grouped := make(map[uuid.UUID][]core.CronjobStatsDaily)
	for _, stat := range stats {
		grouped[stat.CronjobID] = append(grouped[stat.CronjobID], stat)
	}

	return grouped, nil
}
