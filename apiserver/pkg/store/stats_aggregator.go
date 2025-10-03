package store

import (
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// StatsAggregatorStore 定义统计数据聚合存储接口
// 负责执行统计数据聚合的数据库操作
type StatsAggregatorStore interface {
	// AggregateTaskStats 聚合任务统计数据
	AggregateTaskStats(targetDate string) (affectedRows int64, skippedNullTeam int64, err error)

	// AggregateCronjobStats 聚合CronJob统计数据
	AggregateCronjobStats(targetDate string) (affectedRows int64, err error)

	// AggregateWorkerStats 聚合Worker统计数据
	AggregateWorkerStats(targetDate string) (affectedRows int64, err error)

	// CheckNullTeamCount 检查指定日期有多少 team_id 为 NULL 的任务
	CheckNullTeamCount(targetDate string) (count int64, err error)
}

// statsAggregatorStore GORM 实现
type statsAggregatorStore struct {
	db *gorm.DB
}

// NewStatsAggregatorStore 创建 StatsAggregatorStore 实例
func NewStatsAggregatorStore(db *gorm.DB) StatsAggregatorStore {
	return &statsAggregatorStore{db: db}
}

// CheckNullTeamCount 检查指定日期有多少 team_id 为 NULL 的任务
func (s *statsAggregatorStore) CheckNullTeamCount(targetDate string) (int64, error) {
	var count int64
	sql := `
	SELECT COUNT(*) 
	FROM tasks 
	WHERE DATE(time_end) = ? 
	  AND deleted = 0 
	  AND team_id IS NULL
	  AND time_start IS NOT NULL
	  AND time_end IS NOT NULL
	`
	if err := s.db.Raw(sql, targetDate).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// AggregateTaskStats 聚合任务统计数据
//
// 🔥 严格模式：只聚合有 team_id 的任务
// - 如果 tasks.team_id 为 NULL，会被跳过
// - 统计结果按 team_id 分组存储
func (s *statsAggregatorStore) AggregateTaskStats(targetDate string) (int64, int64, error) {
	// 🔥 先检查是否有 team_id 为 NULL 的任务
	nullTeamCount, err := s.CheckNullTeamCount(targetDate)
	if err != nil {
		logger.Warn("检查 NULL team_id 失败", zap.Error(err))
	} else if nullTeamCount > 0 {
		logger.Warn("发现 team_id 为 NULL 的任务，这些任务将被跳过",
			zap.String("date", targetDate),
			zap.Int64("count", nullTeamCount))
	}

	// 🔥 使用原生SQL聚合，性能最优
	// 注意：只聚合 team_id 不为 NULL 的任务
	// 🔥 Bug修复：使用 UUID() 函数为每个 team 生成不同的 id
	sql := `
	INSERT INTO task_stats_daily 
		(id, team_id, stat_date, total_tasks, success_tasks, failed_tasks, 
		 error_tasks, timeout_tasks, avg_duration, min_duration, max_duration,
		 created_at, updated_at)
	SELECT 
		UUID() as id,
		team_id,
		DATE(time_end) as stat_date,
		COUNT(*) as total_tasks,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_tasks,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_tasks,
		SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error_tasks,
		SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END) as timeout_tasks,
		COALESCE(AVG(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as avg_duration,
		COALESCE(MIN(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as min_duration,
		COALESCE(MAX(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as max_duration,
		NOW() as created_at,
		NOW() as updated_at
	FROM tasks
	WHERE DATE(time_end) = ?
	  AND deleted = 0
	  AND team_id IS NOT NULL
	  AND time_start IS NOT NULL
	  AND time_end IS NOT NULL
	GROUP BY team_id, DATE(time_end)
	ON DUPLICATE KEY UPDATE
		total_tasks = VALUES(total_tasks),
		success_tasks = VALUES(success_tasks),
		failed_tasks = VALUES(failed_tasks),
		error_tasks = VALUES(error_tasks),
		timeout_tasks = VALUES(timeout_tasks),
		avg_duration = VALUES(avg_duration),
		min_duration = VALUES(min_duration),
		max_duration = VALUES(max_duration),
		updated_at = NOW()
	`

	result := s.db.Exec(sql, targetDate)
	if result.Error != nil {
		return 0, nullTeamCount, result.Error
	}

	return result.RowsAffected, nullTeamCount, nil
}

// AggregateCronjobStats 聚合CronJob统计数据
//
// 🔥 严格模式：只聚合有 team_id 的任务
// 🔥 Bug修复：使用 UUID() 函数为每个 cronjob 生成不同的 id
// 🔥 优化：冗余存储 cronjob_name，提升查询性能（避免JOIN）
func (s *statsAggregatorStore) AggregateCronjobStats(targetDate string) (int64, error) {
	// 注意：只聚合 team_id 不为 NULL 的任务
	// 使用 LEFT JOIN 获取 cronjob 名称
	sql := `
	INSERT INTO cronjob_stats_daily 
		(id, cronjob_id, cronjob_name, team_id, stat_date, total_tasks, success_tasks, failed_tasks, 
		 error_tasks, timeout_tasks, avg_duration, min_duration, max_duration,
		 created_at, updated_at)
	SELECT 
		UUID() as id,
		t.cronjob,
		COALESCE(c.name, 'Unknown') as cronjob_name,
		t.team_id,
		DATE(t.time_end) as stat_date,
		COUNT(*) as total_tasks,
		SUM(CASE WHEN t.status = 'success' THEN 1 ELSE 0 END) as success_tasks,
		SUM(CASE WHEN t.status = 'failed' THEN 1 ELSE 0 END) as failed_tasks,
		SUM(CASE WHEN t.status = 'error' THEN 1 ELSE 0 END) as error_tasks,
		SUM(CASE WHEN t.status = 'timeout' THEN 1 ELSE 0 END) as timeout_tasks,
		COALESCE(AVG(TIMESTAMPDIFF(SECOND, t.time_start, t.time_end)), 0) as avg_duration,
		COALESCE(MIN(TIMESTAMPDIFF(SECOND, t.time_start, t.time_end)), 0) as min_duration,
		COALESCE(MAX(TIMESTAMPDIFF(SECOND, t.time_start, t.time_end)), 0) as max_duration,
		NOW() as created_at,
		NOW() as updated_at
	FROM tasks t
	LEFT JOIN cronjobs c ON t.cronjob = c.id AND c.deleted = 0
	WHERE DATE(t.time_end) = ?
	  AND t.deleted = 0
	  AND t.cronjob IS NOT NULL
	  AND t.team_id IS NOT NULL
	  AND t.time_start IS NOT NULL
	  AND t.time_end IS NOT NULL
	GROUP BY t.cronjob, t.team_id, DATE(t.time_end)
	ON DUPLICATE KEY UPDATE
		cronjob_name = VALUES(cronjob_name),
		total_tasks = VALUES(total_tasks),
		success_tasks = VALUES(success_tasks),
		failed_tasks = VALUES(failed_tasks),
		error_tasks = VALUES(error_tasks),
		timeout_tasks = VALUES(timeout_tasks),
		avg_duration = VALUES(avg_duration),
		min_duration = VALUES(min_duration),
		max_duration = VALUES(max_duration),
		updated_at = NOW()
	`

	result := s.db.Exec(sql, targetDate)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// AggregateWorkerStats 聚合Worker统计数据
//
// 🔥 严格模式：只聚合有 team_id 的任务
// 🔥 Bug修复：使用 UUID() 函数为每个 worker 生成不同的 id
func (s *statsAggregatorStore) AggregateWorkerStats(targetDate string) (int64, error) {
	sql := `
	INSERT INTO worker_stats_daily 
		(id, worker_id, team_id, worker_name, stat_date, total_tasks, success_tasks, failed_tasks, 
		 error_tasks, timeout_tasks, avg_duration, min_duration, max_duration,
		 created_at, updated_at)
	SELECT 
		UUID() as id,
		worker_id,
		team_id,
		worker_name,
		DATE(time_end) as stat_date,
		COUNT(*) as total_tasks,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_tasks,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_tasks,
		SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as error_tasks,
		SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END) as timeout_tasks,
		COALESCE(AVG(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as avg_duration,
		COALESCE(MIN(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as min_duration,
		COALESCE(MAX(TIMESTAMPDIFF(SECOND, time_start, time_end)), 0) as max_duration,
		NOW() as created_at,
		NOW() as updated_at
	FROM tasks
	WHERE DATE(time_end) = ?
	  AND deleted = 0
	  AND worker_id IS NOT NULL
	  AND team_id IS NOT NULL
	  AND time_start IS NOT NULL
	  AND time_end IS NOT NULL
	GROUP BY worker_id, team_id, worker_name, DATE(time_end)
	ON DUPLICATE KEY UPDATE
		total_tasks = VALUES(total_tasks),
		success_tasks = VALUES(success_tasks),
		failed_tasks = VALUES(failed_tasks),
		error_tasks = VALUES(error_tasks),
		timeout_tasks = VALUES(timeout_tasks),
		avg_duration = VALUES(avg_duration),
		min_duration = VALUES(min_duration),
		max_duration = VALUES(max_duration),
		updated_at = NOW()
	`

	result := s.db.Exec(sql, targetDate)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
