package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/middleware"
	"github.com/codelieche/todolist/pkg/utils/controllers"
	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StatsAnalysisController 统计分析控制器
// 提供深度数据分析和趋势统计
type StatsAnalysisController struct {
	controllers.BaseController
	service core.TodoListService
}

// NewStatsAnalysisController 创建统计分析控制器实例
func NewStatsAnalysisController(service core.TodoListService) *StatsAnalysisController {
	return &StatsAnalysisController{
		service: service,
	}
}

// GetAnalysis 获取统计分析数据
// @Summary 获取待办事项统计分析
// @Description 获取待办事项的深度统计分析，包括完成率趋势、时间分布、分类统计等
// @Tags TodoList
// @Accept json
// @Produce json
// @Param days query int false "统计天数" default(30)
// @Success 200 {object} map[string]interface{} "统计分析数据"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 500 {object} core.ErrorResponse "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/analysis/ [get]
func (ctrl *StatsAnalysisController) GetAnalysis(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取统计天数参数（默认30天）
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := fmt.Sscanf(daysStr, "%d", &days); err == nil && d == 1 {
			if days < 1 {
				days = 30
			}
			if days > 365 {
				days = 365
			}
		}
	}

	// 构建基础过滤器（用户ID + 团队过滤）
	userFilter := &filters.FilterOption{
		Column: "user_id",
		Value:  user.UserID,
		Op:     filters.FILTER_EQ,
	}
	baseFilters := []filters.Filter{userFilter}
	baseFilters = ctrl.AppendTeamFilter(c, baseFilters)

	// 准备返回数据
	analysis := make(map[string]interface{})

	// ========== 1. 完成率趋势分析 ==========
	completionTrend, err := ctrl.getCompletionTrend(ctx, baseFilters, days)
	if err != nil {
		logger.Error("get completion trend error", zap.Error(err))
	}
	analysis["completion_trend"] = completionTrend

	// ========== 2. 时间分布分析 ==========
	timeDistribution, err := ctrl.getTimeDistribution(ctx, baseFilters, days)
	if err != nil {
		logger.Error("get time distribution error", zap.Error(err))
	}
	analysis["time_distribution"] = timeDistribution

	// ========== 3. 分类统计分析 ==========
	categoryStats, err := ctrl.getCategoryStats(ctx, baseFilters)
	if err != nil {
		logger.Error("get category stats error", zap.Error(err))
	}
	analysis["category_stats"] = categoryStats

	// ========== 4. 时间段对比 ==========
	periodComparison, err := ctrl.getPeriodComparison(ctx, baseFilters)
	if err != nil {
		logger.Error("get period comparison error", zap.Error(err))
	}
	analysis["period_comparison"] = periodComparison

	// ========== 5. 完成时长分析 ==========
	completionTimeStats, err := ctrl.getCompletionTimeStats(ctx, baseFilters, days)
	if err != nil {
		logger.Error("get completion time stats error", zap.Error(err))
	}
	analysis["completion_time_stats"] = completionTimeStats

	ctrl.HandleOK(c, analysis)
}

// getCompletionTrend 获取完成率趋势（按天统计）
func (ctrl *StatsAnalysisController) getCompletionTrend(ctx context.Context, baseFilters []filters.Filter, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	trendData := make([]map[string]interface{}, 0)

	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	// 按天统计
	for i := 0; i < days; i++ {
		dayStart := startDate.AddDate(0, 0, i)
		dayEnd := dayStart.Add(24 * time.Hour)

		// 统计当天创建的任务
		createdFilter := &filters.FilterOption{
			Column: "created_at",
			Value:  dayStart,
			Op:     filters.FILTER_GTE,
		}
		createdEndFilter := &filters.FilterOption{
			Column: "created_at",
			Value:  dayEnd,
			Op:     filters.FILTER_LT,
		}
		dayFilters := append(baseFilters, createdFilter, createdEndFilter)
		created, _ := ctrl.service.Count(ctx, dayFilters...)

		// 统计当天完成的任务
		doneFilter := &filters.FilterOption{
			Column: "finished_at",
			Value:  dayStart,
			Op:     filters.FILTER_GTE,
		}
		doneEndFilter := &filters.FilterOption{
			Column: "finished_at",
			Value:  dayEnd,
			Op:     filters.FILTER_LT,
		}
		statusFilter := &filters.FilterOption{
			Column: "status",
			Value:  core.TodoStatusDone,
			Op:     filters.FILTER_EQ,
		}
		doneFilters := append(baseFilters, doneFilter, doneEndFilter, statusFilter)
		done, _ := ctrl.service.Count(ctx, doneFilters...)

		// 计算完成率
		completionRate := 0.0
		if created > 0 {
			completionRate = float64(done) / float64(created) * 100
		}

		trendData = append(trendData, map[string]interface{}{
			"date":            dayStart.Format("2006-01-02"),
			"created":         created,
			"completed":       done,
			"completion_rate": fmt.Sprintf("%.1f", completionRate),
		})
	}

	result["data"] = trendData
	result["days"] = days
	return result, nil
}

// getTimeDistribution 获取时间分布（按小时和星期几）
func (ctrl *StatsAnalysisController) getTimeDistribution(ctx context.Context, baseFilters []filters.Filter, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 获取最近N天的所有任务
	startDate := time.Now().AddDate(0, 0, -days)
	dateFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  startDate,
		Op:     filters.FILTER_GTE,
	}
	recentFilters := append(baseFilters, dateFilter)

	// 获取任务列表（限制1000条，避免数据过多）
	todos, err := ctrl.service.List(ctx, 0, 1000, recentFilters...)
	if err != nil {
		return result, err
	}

	// 按小时统计（0-23）
	hourlyCreated := make([]int, 24)
	hourlyCompleted := make([]int, 24)

	// 按星期几统计（0=周日, 1=周一, ..., 6=周六）
	weekdayCreated := make([]int, 7)
	weekdayCompleted := make([]int, 7)

	for _, todo := range todos {
		// 创建时间分布
		hour := todo.CreatedAt.Hour()
		weekday := int(todo.CreatedAt.Weekday())
		hourlyCreated[hour]++
		weekdayCreated[weekday]++

		// 完成时间分布
		if todo.Status == core.TodoStatusDone && todo.FinishedAt != nil {
			completedHour := todo.FinishedAt.Hour()
			completedWeekday := int(todo.FinishedAt.Weekday())
			hourlyCompleted[completedHour]++
			weekdayCompleted[completedWeekday]++
		}
	}

	// 构建返回数据
	hourlyData := make([]map[string]interface{}, 24)
	for i := 0; i < 24; i++ {
		hourlyData[i] = map[string]interface{}{
			"hour":      i,
			"created":   hourlyCreated[i],
			"completed": hourlyCompleted[i],
		}
	}

	weekdayNames := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	weekdayData := make([]map[string]interface{}, 7)
	for i := 0; i < 7; i++ {
		weekdayData[i] = map[string]interface{}{
			"weekday":   weekdayNames[i],
			"created":   weekdayCreated[i],
			"completed": weekdayCompleted[i],
		}
	}

	result["hourly"] = hourlyData
	result["weekday"] = weekdayData
	return result, nil
}

// getCategoryStats 获取分类统计
func (ctrl *StatsAnalysisController) getCategoryStats(ctx context.Context, baseFilters []filters.Filter) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 获取所有任务（限制2000条）
	todos, err := ctrl.service.List(ctx, 0, 2000, baseFilters...)
	if err != nil {
		return result, err
	}

	// 按分类统计
	categoryMap := make(map[string]map[string]int)

	for _, todo := range todos {
		category := todo.Category
		if category == "" {
			category = "general"
		}

		if _, exists := categoryMap[category]; !exists {
			categoryMap[category] = map[string]int{
				"total":    0,
				"done":     0,
				"pending":  0,
				"running":  0,
				"canceled": 0,
			}
		}

		categoryMap[category]["total"]++
		switch todo.Status {
		case core.TodoStatusDone:
			categoryMap[category]["done"]++
		case core.TodoStatusPending:
			categoryMap[category]["pending"]++
		case core.TodoStatusRunning:
			categoryMap[category]["running"]++
		case core.TodoStatusCanceled:
			categoryMap[category]["canceled"]++
		}
	}

	// 转换为数组格式
	categoryData := make([]map[string]interface{}, 0)
	for category, stats := range categoryMap {
		total := stats["total"]
		done := stats["done"]
		completionRate := 0.0
		if total > 0 {
			completionRate = float64(done) / float64(total) * 100
		}

		categoryData = append(categoryData, map[string]interface{}{
			"category":        category,
			"total":           total,
			"done":            done,
			"pending":         stats["pending"],
			"running":         stats["running"],
			"canceled":        stats["canceled"],
			"completion_rate": fmt.Sprintf("%.1f", completionRate),
		})
	}

	result["data"] = categoryData
	return result, nil
}

// getPeriodComparison 获取时间段对比（本周 vs 上周，本月 vs 上月）
func (ctrl *StatsAnalysisController) getPeriodComparison(ctx context.Context, baseFilters []filters.Filter) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	now := time.Now()

	// ========== 本周 vs 上周 ==========
	// 本周开始时间（周一）
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // 周日算作第7天
	}
	thisWeekStart := now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	// 统计本周
	thisWeekFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  thisWeekStart,
		Op:     filters.FILTER_GTE,
	}
	thisWeekFilters := append(baseFilters, thisWeekFilter)
	thisWeekCreated, _ := ctrl.service.Count(ctx, thisWeekFilters...)

	thisWeekDoneFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  thisWeekStart,
		Op:     filters.FILTER_GTE,
	}
	thisWeekStatusFilter := &filters.FilterOption{
		Column: "status",
		Value:  core.TodoStatusDone,
		Op:     filters.FILTER_EQ,
	}
	thisWeekDoneFilters := append(baseFilters, thisWeekDoneFilter, thisWeekStatusFilter)
	thisWeekDone, _ := ctrl.service.Count(ctx, thisWeekDoneFilters...)

	// 统计上周
	lastWeekStartFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  lastWeekStart,
		Op:     filters.FILTER_GTE,
	}
	lastWeekEndFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  lastWeekEnd,
		Op:     filters.FILTER_LT,
	}
	lastWeekFilters := append(baseFilters, lastWeekStartFilter, lastWeekEndFilter)
	lastWeekCreated, _ := ctrl.service.Count(ctx, lastWeekFilters...)

	lastWeekDoneStartFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  lastWeekStart,
		Op:     filters.FILTER_GTE,
	}
	lastWeekDoneEndFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  lastWeekEnd,
		Op:     filters.FILTER_LT,
	}
	lastWeekDoneFilters := append(baseFilters, lastWeekDoneStartFilter, lastWeekDoneEndFilter, thisWeekStatusFilter)
	lastWeekDone, _ := ctrl.service.Count(ctx, lastWeekDoneFilters...)

	// ========== 本月 vs 上月 ==========
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)
	lastMonthEnd := thisMonthStart

	// 统计本月
	thisMonthFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  thisMonthStart,
		Op:     filters.FILTER_GTE,
	}
	thisMonthFilters := append(baseFilters, thisMonthFilter)
	thisMonthCreated, _ := ctrl.service.Count(ctx, thisMonthFilters...)

	thisMonthDoneFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  thisMonthStart,
		Op:     filters.FILTER_GTE,
	}
	thisMonthDoneFilters := append(baseFilters, thisMonthDoneFilter, thisWeekStatusFilter)
	thisMonthDone, _ := ctrl.service.Count(ctx, thisMonthDoneFilters...)

	// 统计上月
	lastMonthStartFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  lastMonthStart,
		Op:     filters.FILTER_GTE,
	}
	lastMonthEndFilter := &filters.FilterOption{
		Column: "created_at",
		Value:  lastMonthEnd,
		Op:     filters.FILTER_LT,
	}
	lastMonthFilters := append(baseFilters, lastMonthStartFilter, lastMonthEndFilter)
	lastMonthCreated, _ := ctrl.service.Count(ctx, lastMonthFilters...)

	lastMonthDoneStartFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  lastMonthStart,
		Op:     filters.FILTER_GTE,
	}
	lastMonthDoneEndFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  lastMonthEnd,
		Op:     filters.FILTER_LT,
	}
	lastMonthDoneFilters := append(baseFilters, lastMonthDoneStartFilter, lastMonthDoneEndFilter, thisWeekStatusFilter)
	lastMonthDone, _ := ctrl.service.Count(ctx, lastMonthDoneFilters...)

	// 计算完成率
	thisWeekRate := 0.0
	if thisWeekCreated > 0 {
		thisWeekRate = float64(thisWeekDone) / float64(thisWeekCreated) * 100
	}
	lastWeekRate := 0.0
	if lastWeekCreated > 0 {
		lastWeekRate = float64(lastWeekDone) / float64(lastWeekCreated) * 100
	}
	thisMonthRate := 0.0
	if thisMonthCreated > 0 {
		thisMonthRate = float64(thisMonthDone) / float64(thisMonthCreated) * 100
	}
	lastMonthRate := 0.0
	if lastMonthCreated > 0 {
		lastMonthRate = float64(lastMonthDone) / float64(lastMonthCreated) * 100
	}

	result["weekly"] = map[string]interface{}{
		"this_week": map[string]interface{}{
			"created":         thisWeekCreated,
			"completed":       thisWeekDone,
			"completion_rate": fmt.Sprintf("%.1f", thisWeekRate),
		},
		"last_week": map[string]interface{}{
			"created":         lastWeekCreated,
			"completed":       lastWeekDone,
			"completion_rate": fmt.Sprintf("%.1f", lastWeekRate),
		},
	}

	result["monthly"] = map[string]interface{}{
		"this_month": map[string]interface{}{
			"created":         thisMonthCreated,
			"completed":       thisMonthDone,
			"completion_rate": fmt.Sprintf("%.1f", thisMonthRate),
		},
		"last_month": map[string]interface{}{
			"created":         lastMonthCreated,
			"completed":       lastMonthDone,
			"completion_rate": fmt.Sprintf("%.1f", lastMonthRate),
		},
	}

	return result, nil
}

// getCompletionTimeStats 获取完成时长统计
func (ctrl *StatsAnalysisController) getCompletionTimeStats(ctx context.Context, baseFilters []filters.Filter, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 获取最近N天已完成的任务
	startDate := time.Now().AddDate(0, 0, -days)
	dateFilter := &filters.FilterOption{
		Column: "finished_at",
		Value:  startDate,
		Op:     filters.FILTER_GTE,
	}
	statusFilter := &filters.FilterOption{
		Column: "status",
		Value:  core.TodoStatusDone,
		Op:     filters.FILTER_EQ,
	}
	recentFilters := append(baseFilters, dateFilter, statusFilter)

	// 获取已完成任务列表（限制1000条）
	todos, err := ctrl.service.List(ctx, 0, 1000, recentFilters...)
	if err != nil {
		return result, err
	}

	// 统计完成时长分布
	var totalDuration int64 // 总时长（秒）
	durationRanges := map[string]int{
		"1h":  0, // 1小时内
		"1d":  0, // 1天内
		"3d":  0, // 3天内
		"1w":  0, // 1周内
		"1m":  0, // 1月内
		"1m+": 0, // 1月以上
	}

	validCount := 0
	for _, todo := range todos {
		if todo.FinishedAt == nil {
			continue
		}

		// 计算从创建到完成的时长
		duration := todo.FinishedAt.Sub(todo.CreatedAt)
		if duration < 0 {
			continue // 跳过异常数据
		}

		validCount++
		totalDuration += int64(duration.Seconds())

		// 分类统计
		hours := duration.Hours()
		if hours <= 1 {
			durationRanges["1h"]++
		} else if hours <= 24 {
			durationRanges["1d"]++
		} else if hours <= 72 {
			durationRanges["3d"]++
		} else if hours <= 168 {
			durationRanges["1w"]++
		} else if hours <= 720 {
			durationRanges["1m"]++
		} else {
			durationRanges["1m+"]++
		}
	}

	// 计算平均完成时长
	avgDuration := 0.0
	if validCount > 0 {
		avgDuration = float64(totalDuration) / float64(validCount) / 3600 // 转换为小时
	}

	result["average_hours"] = fmt.Sprintf("%.1f", avgDuration)
	result["total_completed"] = validCount
	result["distribution"] = []map[string]interface{}{
		{"range": "1小时内", "count": durationRanges["1h"]},
		{"range": "1天内", "count": durationRanges["1d"]},
		{"range": "3天内", "count": durationRanges["3d"]},
		{"range": "1周内", "count": durationRanges["1w"]},
		{"range": "1月内", "count": durationRanges["1m"]},
		{"range": "1月以上", "count": durationRanges["1m+"]},
	}

	return result, nil
}
