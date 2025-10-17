package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WorkflowStatsService Workflow统计服务
//
// 实现 core.WorkflowStatsService 接口
type WorkflowStatsService struct {
	db            *gorm.DB
	statsStore    *store.WorkflowStatsStore
	executeStore  core.WorkflowExecuteStore
	workflowStore core.WorkflowStore
}

// NewWorkflowStatsService 创建Service实例
func NewWorkflowStatsService(
	db *gorm.DB,
	statsStore *store.WorkflowStatsStore,
	executeStore core.WorkflowExecuteStore,
	workflowStore core.WorkflowStore,
) *WorkflowStatsService {
	return &WorkflowStatsService{
		db:            db,
		statsStore:    statsStore,
		executeStore:  executeStore,
		workflowStore: workflowStore,
	}
}

// DailyAgg 每日聚合数据（辅助结构）
type DailyAgg struct {
	Date     string
	Total    int
	Success  int
	Failed   int
	Canceled int
}

// AggregateDailyStats 聚合指定日期的统计数据
// 从 workflow_executes 表聚合到 workflow_stats_daily 表
func (s *WorkflowStatsService) AggregateDailyStats(ctx context.Context, date time.Time) error {
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	endTime := startTime.Add(24 * time.Hour)

	logger.Info("开始聚合Workflow统计数据",
		zap.String("date", date.Format("2006-01-02")),
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime))

	// 🔥 使用原生SQL聚合查询（性能优化）
	// 按 workflow_id, team_id 分组聚合
	query := `
		SELECT 
			workflow_id,
			team_id,
			COUNT(*) as total_executes,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_executes,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_executes,
			SUM(CASE WHEN status = 'canceled' THEN 1 ELSE 0 END) as canceled_executes,
			AVG(CASE 
				WHEN time_start IS NOT NULL AND time_end IS NOT NULL 
				THEN TIMESTAMPDIFF(SECOND, time_start, time_end) 
				ELSE NULL 
			END) as avg_duration,
			MIN(CASE 
				WHEN time_start IS NOT NULL AND time_end IS NOT NULL 
				THEN TIMESTAMPDIFF(SECOND, time_start, time_end) 
				ELSE NULL 
			END) as min_duration,
			MAX(CASE 
				WHEN time_start IS NOT NULL AND time_end IS NOT NULL 
				THEN TIMESTAMPDIFF(SECOND, time_start, time_end) 
				ELSE NULL 
			END) as max_duration,
			AVG(total_steps) as avg_total_steps,
			AVG(success_steps) as avg_success_steps,
			AVG(failed_steps) as avg_failed_steps,
			SUM(CASE WHEN trigger_type = 'manual' THEN 1 ELSE 0 END) as manual_triggers,
			SUM(CASE WHEN trigger_type = 'api' THEN 1 ELSE 0 END) as api_triggers,
			SUM(CASE WHEN trigger_type = 'webhook' THEN 1 ELSE 0 END) as webhook_triggers,
			SUM(CASE WHEN trigger_type = 'schedule' THEN 1 ELSE 0 END) as schedule_triggers
		FROM workflow_executes
		WHERE created_at >= ? AND created_at < ?
			AND deleted = 0
			AND status IN ('success', 'failed', 'canceled')
		GROUP BY workflow_id, team_id
	`

	type AggResult struct {
		WorkflowID       string   `gorm:"column:workflow_id"`
		TeamID           *string  `gorm:"column:team_id"`
		TotalExecutes    int      `gorm:"column:total_executes"`
		SuccessExecutes  int      `gorm:"column:success_executes"`
		FailedExecutes   int      `gorm:"column:failed_executes"`
		CanceledExecutes int      `gorm:"column:canceled_executes"`
		AvgDuration      *float64 `gorm:"column:avg_duration"`
		MinDuration      *float64 `gorm:"column:min_duration"`
		MaxDuration      *float64 `gorm:"column:max_duration"`
		AvgTotalSteps    *float64 `gorm:"column:avg_total_steps"`
		AvgSuccessSteps  *float64 `gorm:"column:avg_success_steps"`
		AvgFailedSteps   *float64 `gorm:"column:avg_failed_steps"`
		ManualTriggers   int      `gorm:"column:manual_triggers"`
		ApiTriggers      int      `gorm:"column:api_triggers"`
		WebhookTriggers  int      `gorm:"column:webhook_triggers"`
		ScheduleTriggers int      `gorm:"column:schedule_triggers"`
	}

	var results []AggResult
	if err := s.db.WithContext(ctx).Raw(query, startTime, endTime).Scan(&results).Error; err != nil {
		logger.Error("聚合查询失败", zap.Error(err))
		return err
	}

	logger.Info("聚合查询完成", zap.Int("workflow_count", len(results)))

	// 遍历聚合结果，写入统计表
	for _, result := range results {
		workflowID, err := uuid.Parse(result.WorkflowID)
		if err != nil {
			logger.Error("解析workflow_id失败", zap.String("workflow_id", result.WorkflowID), zap.Error(err))
			continue
		}

		var teamID *uuid.UUID
		if result.TeamID != nil && *result.TeamID != "" {
			tid, err := uuid.Parse(*result.TeamID)
			if err == nil {
				teamID = &tid
			}
		}

		// 查询Workflow名称
		workflow, err := s.workflowStore.FindByID(ctx, workflowID)
		workflowName := ""
		if err == nil {
			workflowName = workflow.Name
		}

		// 查询是否已存在统计记录
		existingStats, err := s.statsStore.FindByWorkflowAndDate(ctx, workflowID, teamID, date)

		stats := &core.WorkflowStatsDaily{
			WorkflowID:       workflowID,
			WorkflowName:     workflowName,
			TeamID:           teamID,
			StatDate:         startTime,
			TotalExecutes:    result.TotalExecutes,
			SuccessExecutes:  result.SuccessExecutes,
			FailedExecutes:   result.FailedExecutes,
			CanceledExecutes: result.CanceledExecutes,
			ManualTriggers:   result.ManualTriggers,
			ApiTriggers:      result.ApiTriggers,
			WebhookTriggers:  result.WebhookTriggers,
			ScheduleTriggers: result.ScheduleTriggers,
		}

		// 处理可能为空的平均值
		if result.AvgDuration != nil {
			stats.AvgDuration = *result.AvgDuration
		}
		if result.MinDuration != nil {
			stats.MinDuration = *result.MinDuration
		}
		if result.MaxDuration != nil {
			stats.MaxDuration = *result.MaxDuration
		}
		if result.AvgTotalSteps != nil {
			stats.AvgTotalSteps = *result.AvgTotalSteps
		}
		if result.AvgSuccessSteps != nil {
			stats.AvgSuccessSteps = *result.AvgSuccessSteps
		}
		if result.AvgFailedSteps != nil {
			stats.AvgFailedSteps = *result.AvgFailedSteps
		}

		// 如果已存在，更新；否则创建
		if existingStats != nil {
			stats.ID = existingStats.ID
			if err := s.statsStore.Update(ctx, stats); err != nil {
				logger.Error("更新统计记录失败",
					zap.String("workflow_id", workflowID.String()),
					zap.Error(err))
			}
		} else {
			if err := s.statsStore.Create(ctx, stats); err != nil {
				logger.Error("创建统计记录失败",
					zap.String("workflow_id", workflowID.String()),
					zap.Error(err))
			}
		}
	}

	logger.Info("Workflow统计数据聚合完成",
		zap.String("date", date.Format("2006-01-02")),
		zap.Int("processed_workflows", len(results)))

	return nil
}

// AggregateHistoricalStats 聚合历史统计数据（批量）
// 用于初次部署或补充历史数据
func (s *WorkflowStatsService) AggregateHistoricalStats(ctx context.Context, startDate, endDate time.Time) error {
	logger.Info("开始批量聚合历史统计数据",
		zap.String("start_date", startDate.Format("2006-01-02")),
		zap.String("end_date", endDate.Format("2006-01-02")))

	// 逐日聚合
	currentDate := startDate
	successCount := 0
	failedCount := 0

	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		if err := s.AggregateDailyStats(ctx, currentDate); err != nil {
			logger.Error("聚合单日数据失败",
				zap.String("date", currentDate.Format("2006-01-02")),
				zap.Error(err))
			failedCount++
		} else {
			successCount++
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	logger.Info("批量聚合完成",
		zap.Int("success_count", successCount),
		zap.Int("failed_count", failedCount))

	return nil
}

// GetSuccessRateTrend 获取成功率趋势
// 返回最近N天每天的执行统计（total, success, failed, success_rate）
func (s *WorkflowStatsService) GetSuccessRateTrend(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
) (map[string]interface{}, error) {
	stats, err := s.statsStore.GetDailyStats(ctx, teamID, days)
	if err != nil {
		return nil, err
	}

	// 聚合每日数据
	dailyMap := make(map[string]*DailyAgg)
	for _, stat := range stats {
		dateKey := stat.StatDate.Format("2006-01-02")
		if agg, exists := dailyMap[dateKey]; exists {
			agg.Total += stat.TotalExecutes
			agg.Success += stat.SuccessExecutes
			agg.Failed += stat.FailedExecutes
			agg.Canceled += stat.CanceledExecutes
		} else {
			dailyMap[dateKey] = &DailyAgg{
				Date:     dateKey,
				Total:    stat.TotalExecutes,
				Success:  stat.SuccessExecutes,
				Failed:   stat.FailedExecutes,
				Canceled: stat.CanceledExecutes,
			}
		}
	}

	// 转换为列表并排序
	var trendData []map[string]interface{}
	dates := make([]string, 0, len(dailyMap))
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	for _, date := range dates {
		agg := dailyMap[date]
		successRate := 0.0
		if agg.Total > 0 {
			successRate = float64(agg.Success) * 100.0 / float64(agg.Total)
		}

		trendData = append(trendData, map[string]interface{}{
			"date":         agg.Date,
			"total":        agg.Total,
			"success":      agg.Success,
			"failed":       agg.Failed,
			"canceled":     agg.Canceled,
			"success_rate": fmt.Sprintf("%.1f", successRate),
		})
	}

	return map[string]interface{}{
		"data": trendData,
	}, nil
}

// GetExecutionEfficiency 获取执行效率统计
func (s *WorkflowStatsService) GetExecutionEfficiency(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
) (map[string]interface{}, error) {
	stats, err := s.statsStore.GetDailyStats(ctx, teamID, days)
	if err != nil {
		return nil, err
	}

	// 计算平均值
	totalExecutes := 0
	totalDuration := 0.0
	totalSuccess := 0
	totalSuccessDuration := 0.0

	for _, stat := range stats {
		totalExecutes += stat.TotalExecutes
		totalDuration += stat.AvgDuration * float64(stat.TotalExecutes)
		totalSuccess += stat.SuccessExecutes
		// 假设成功任务的平均时长与总平均时长相同（简化计算）
		totalSuccessDuration += stat.AvgDuration * float64(stat.SuccessExecutes)
	}

	avgDuration := 0.0
	avgSuccessDuration := 0.0
	if totalExecutes > 0 {
		avgDuration = totalDuration / float64(totalExecutes)
	}
	if totalSuccess > 0 {
		avgSuccessDuration = totalSuccessDuration / float64(totalSuccess)
	}

	return map[string]interface{}{
		"average_duration":         fmt.Sprintf("%.2f", avgDuration),
		"average_success_duration": fmt.Sprintf("%.2f", avgSuccessDuration),
		"total_executed":           totalExecutes,
		"total_success":            totalSuccess,
	}, nil
}

// GetWorkflowRanking 获取Workflow排行榜
func (s *WorkflowStatsService) GetWorkflowRanking(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
) (map[string]interface{}, error) {
	ranking, err := s.statsStore.GetWorkflowRanking(ctx, teamID, days, 10)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data":           ranking,
		"workflow_count": len(ranking),
	}, nil
}

// GetTimeDistribution 获取时间分布统计
// 返回按星期几的执行分布
func (s *WorkflowStatsService) GetTimeDistribution(
	ctx context.Context,
	teamID *uuid.UUID,
	days int,
) (map[string]interface{}, error) {
	stats, err := s.statsStore.GetDailyStats(ctx, teamID, days)
	if err != nil {
		return nil, err
	}

	// 按星期几统计
	weekdayMap := make(map[string]*DailyAgg)
	weekdayNames := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

	for _, stat := range stats {
		weekday := int(stat.StatDate.Weekday())
		if weekday == 0 {
			weekday = 7 // 周日
		}
		weekdayName := weekdayNames[weekday-1]

		if agg, exists := weekdayMap[weekdayName]; exists {
			agg.Total += stat.TotalExecutes
			agg.Success += stat.SuccessExecutes
			agg.Failed += stat.FailedExecutes
		} else {
			weekdayMap[weekdayName] = &DailyAgg{
				Date:    weekdayName,
				Total:   stat.TotalExecutes,
				Success: stat.SuccessExecutes,
				Failed:  stat.FailedExecutes,
			}
		}
	}

	// 转换为列表（保持顺序）
	var weekdayData []map[string]interface{}
	for _, name := range weekdayNames {
		if agg, exists := weekdayMap[name]; exists {
			weekdayData = append(weekdayData, map[string]interface{}{
				"weekday":  name,
				"executed": agg.Total,
				"success":  agg.Success,
				"failed":   agg.Failed,
			})
		}
	}

	return map[string]interface{}{
		"weekday": weekdayData,
	}, nil
}

// GetPeriodComparison 获取时间段对比
// 返回本周vs上周、本月vs上月的对比数据
func (s *WorkflowStatsService) GetPeriodComparison(
	ctx context.Context,
	teamID *uuid.UUID,
) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	now := time.Now()

	// ========== 本周 vs 上周 ==========
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	thisWeekStart := now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	// 统计本周
	thisWeekStats, _ := s.statsStore.GetAggregateStats(ctx, teamID, thisWeekStart, now)
	// 统计上周
	lastWeekStats, _ := s.statsStore.GetAggregateStats(ctx, teamID, lastWeekStart, lastWeekEnd)

	result["weekly"] = map[string]interface{}{
		"this_week": thisWeekStats,
		"last_week": lastWeekStats,
	}

	// ========== 本月 vs 上月 ==========
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
	lastMonthEnd := thisMonthStart

	// 统计本月
	thisMonthStats, _ := s.statsStore.GetAggregateStats(ctx, teamID, thisMonthStart, now)
	// 统计上月
	lastMonthStats, _ := s.statsStore.GetAggregateStats(ctx, teamID, lastMonthStart, lastMonthEnd)

	result["monthly"] = map[string]interface{}{
		"this_month": thisMonthStats,
		"last_month": lastMonthStats,
	}

	return result, nil
}
