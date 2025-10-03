package services

import (
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// StatsAggregator 统计数据聚合器（Service 层）
//
// 负责统计数据聚合的业务逻辑
// 通过后台定时任务每日凌晨自动执行，大幅提升统计查询性能
//
// 🔥 架构层次：Service -> Store -> Database
// 设计要点：
// - 每日凌晨01:00执行
// - 聚合前一天的数据
// - 支持重复执行（使用 ON DUPLICATE KEY UPDATE）
// - 分别聚合：任务统计、CronJob统计、Worker统计
type StatsAggregator struct {
	aggregatorStore store.StatsAggregatorStore
}

// NewStatsAggregator 创建统计聚合器实例
func NewStatsAggregator(aggregatorStore store.StatsAggregatorStore) *StatsAggregator {
	return &StatsAggregator{
		aggregatorStore: aggregatorStore,
	}
}

// AggregateDailyStats 聚合每日统计数据
//
// 主入口函数，聚合前一天的所有统计数据
// 参数:
//   - targetDate: 目标日期（格式：2006-01-02），如果为空则默认为昨天
//
// 返回值:
//   - error: 如果聚合过程中出现错误则返回错误信息
func (a *StatsAggregator) AggregateDailyStats(targetDate string) error {
	// 如果未指定日期，默认聚合昨天的数据
	if targetDate == "" {
		yesterday := time.Now().AddDate(0, 0, -1)
		targetDate = yesterday.Format("2006-01-02")
	}

	logger.Info("开始聚合每日统计数据",
		zap.String("target_date", targetDate))

	startTime := time.Now()

	// 🔥 1. 聚合任务统计
	if err := a.aggregateTaskStats(targetDate); err != nil {
		logger.Error("聚合任务统计失败",
			zap.String("target_date", targetDate),
			zap.Error(err))
		return fmt.Errorf("聚合任务统计失败: %w", err)
	}

	// 🔥 2. 聚合CronJob统计
	if err := a.aggregateCronjobStats(targetDate); err != nil {
		logger.Error("聚合CronJob统计失败",
			zap.String("target_date", targetDate),
			zap.Error(err))
		return fmt.Errorf("聚合CronJob统计失败: %w", err)
	}

	// 🔥 3. 聚合Worker统计
	if err := a.aggregateWorkerStats(targetDate); err != nil {
		logger.Error("聚合Worker统计失败",
			zap.String("target_date", targetDate),
			zap.Error(err))
		return fmt.Errorf("聚合Worker统计失败: %w", err)
	}

	duration := time.Since(startTime)
	logger.Info("每日统计数据聚合完成",
		zap.String("target_date", targetDate),
		zap.Duration("duration", duration))

	return nil
}

// aggregateTaskStats 聚合任务统计数据（调用 Store 层）
func (a *StatsAggregator) aggregateTaskStats(targetDate string) error {
	logger.Info("开始聚合任务统计", zap.String("date", targetDate))

	// 调用 Store 层执行聚合
	affectedRows, skippedNullTeam, err := a.aggregatorStore.AggregateTaskStats(targetDate)
	if err != nil {
		return err
	}

	logger.Info("任务统计聚合完成",
		zap.String("date", targetDate),
		zap.Int64("affected_rows", affectedRows),
		zap.Int64("skipped_null_team", skippedNullTeam))

	return nil
}

// aggregateCronjobStats 聚合CronJob统计数据（调用 Store 层）
func (a *StatsAggregator) aggregateCronjobStats(targetDate string) error {
	logger.Info("开始聚合CronJob统计", zap.String("date", targetDate))

	// 调用 Store 层执行聚合
	affectedRows, err := a.aggregatorStore.AggregateCronjobStats(targetDate)
	if err != nil {
		return err
	}

	logger.Info("CronJob统计聚合完成",
		zap.String("date", targetDate),
		zap.Int64("affected_rows", affectedRows))

	return nil
}

// aggregateWorkerStats 聚合Worker统计数据（调用 Store 层）
func (a *StatsAggregator) aggregateWorkerStats(targetDate string) error {
	logger.Info("开始聚合Worker统计", zap.String("date", targetDate))

	// 调用 Store 层执行聚合
	affectedRows, err := a.aggregatorStore.AggregateWorkerStats(targetDate)
	if err != nil {
		return err
	}

	logger.Info("Worker统计聚合完成",
		zap.String("date", targetDate),
		zap.Int64("affected_rows", affectedRows))

	return nil
}

// AggregateHistoricalStats 聚合历史统计数据
//
// 用于首次部署或补充历史数据
// 参数:
//   - startDate: 开始日期（格式：2006-01-02）
//   - endDate: 结束日期（格式：2006-01-02）
//
// 返回值:
//   - error: 如果聚合过程中出现错误则返回错误信息
func (a *StatsAggregator) AggregateHistoricalStats(startDate, endDate string) error {
	logger.Info("开始聚合历史统计数据",
		zap.String("start_date", startDate),
		zap.String("end_date", endDate))

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("开始日期格式错误: %w", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("结束日期格式错误: %w", err)
	}

	// 逐天聚合
	successCount := 0
	failCount := 0

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")

		if err := a.AggregateDailyStats(dateStr); err != nil {
			logger.Error("聚合失败",
				zap.String("date", dateStr),
				zap.Error(err))
			failCount++
			// 继续处理下一天，不中断
		} else {
			successCount++
		}
	}

	logger.Info("历史统计数据聚合完成",
		zap.String("start_date", startDate),
		zap.String("end_date", endDate),
		zap.Int("success_count", successCount),
		zap.Int("fail_count", failCount))

	if failCount > 0 {
		return fmt.Errorf("部分日期聚合失败: %d/%d", failCount, successCount+failCount)
	}

	return nil
}
