package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WorkflowStatsController Workflow统计分析控制器
//
// 提供Workflow执行的统计分析功能：
// - 执行成功率趋势
// - 执行效率分析
// - Workflow排行榜
// - 时间分布分析
// - 时间段对比
//
// 架构层次：Controller -> Service -> Store -> Database
type WorkflowStatsController struct {
	controllers.BaseController
	statsService *services.WorkflowStatsService
}

// NewWorkflowStatsController 创建控制器实例
func NewWorkflowStatsController(statsService *services.WorkflowStatsService) *WorkflowStatsController {
	return &WorkflowStatsController{
		statsService: statsService,
	}
}

// GetAnalysis 获取统计分析数据
// @Summary 获取Workflow统计分析
// @Description 获取Workflow执行的深度统计分析，包括成功率趋势、执行效率、排行榜等
// @Tags Workflow
// @Accept json
// @Produce json
// @Param days query int false "统计天数" default(30)
// @Success 200 {object} map[string]interface{} "统计分析数据"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 500 {object} core.ErrorResponse "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /workflow/analysis/ [get]
func (ctrl *WorkflowStatsController) GetAnalysis(c *gin.Context) {
	ctx := c.Request.Context()

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

	// 提取团队ID
	var teamID *uuid.UUID
	if teamIDValue, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
		if tid, ok := teamIDValue.(string); ok && tid != "" {
			if id, err := uuid.Parse(tid); err == nil {
				teamID = &id
			}
		}
	}

	logger.Info("获取Workflow统计分析",
		zap.Int("days", days),
		zap.Any("team_id", teamID))

	// 准备返回数据
	analysis := make(map[string]interface{})

	// ========== 1. 执行成功率趋势 ==========
	successRateTrend, err := ctrl.statsService.GetSuccessRateTrend(ctx, teamID, days)
	if err != nil {
		logger.Error("获取成功率趋势失败", zap.Error(err))
		successRateTrend = map[string]interface{}{"data": []interface{}{}}
	}
	analysis["success_rate_trend"] = successRateTrend

	// ========== 2. 执行效率分析 ==========
	executionEfficiency, err := ctrl.statsService.GetExecutionEfficiency(ctx, teamID, days)
	if err != nil {
		logger.Error("获取执行效率失败", zap.Error(err))
		executionEfficiency = map[string]interface{}{}
	}
	analysis["execution_efficiency"] = executionEfficiency

	// ========== 3. Workflow 排行榜 ==========
	workflowRanking, err := ctrl.statsService.GetWorkflowRanking(ctx, teamID, days)
	if err != nil {
		logger.Error("获取Workflow排行榜失败", zap.Error(err))
		workflowRanking = map[string]interface{}{"data": []interface{}{}}
	}
	analysis["workflow_ranking"] = workflowRanking

	// ========== 4. 时间分布分析 ==========
	timeDistribution, err := ctrl.statsService.GetTimeDistribution(ctx, teamID, days)
	if err != nil {
		logger.Error("获取时间分布失败", zap.Error(err))
		timeDistribution = map[string]interface{}{}
	}
	analysis["time_distribution"] = timeDistribution

	// ========== 5. 时间段对比 ==========
	periodComparison, err := ctrl.statsService.GetPeriodComparison(ctx, teamID)
	if err != nil {
		logger.Error("获取时间段对比失败", zap.Error(err))
		periodComparison = map[string]interface{}{}
	}
	analysis["period_comparison"] = periodComparison

	ctrl.HandleOK(c, analysis)
}

// TriggerDailyAggregation 手动触发每日聚合
// @Summary 手动触发每日统计聚合
// @Description 手动触发指定日期的统计聚合，用于服务挂掉后的数据补偿
// @Tags Workflow
// @Accept json
// @Produce json
// @Param date query string false "聚合日期（格式：2006-01-02，默认为昨天）"
// @Success 200 {object} map[string]interface{} "聚合结果"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 500 {object} core.ErrorResponse "内部错误"
// @Security BearerAuth
// @Router /workflow/stats/aggregate/daily [post]
func (ctrl *WorkflowStatsController) TriggerDailyAggregation(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取日期参数（默认为昨天）
	dateStr := c.Query("date")
	var date time.Time
	var err error

	if dateStr == "" {
		// 默认聚合昨天的数据
		date = time.Now().AddDate(0, 0, -1)
	} else {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
			return
		}
	}

	logger.Info("手动触发每日聚合",
		zap.String("date", date.Format("2006-01-02")))

	startTime := time.Now()

	// 执行聚合
	if err := ctrl.statsService.AggregateDailyStats(ctx, date); err != nil {
		logger.Error("每日聚合失败",
			zap.String("date", date.Format("2006-01-02")),
			zap.Error(err))
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)

	ctrl.HandleOK(c, map[string]interface{}{
		"message":  fmt.Sprintf("✅ 每日聚合完成：%s", date.Format("2006-01-02")),
		"date":     date.Format("2006-01-02"),
		"duration": fmt.Sprintf("%.2fs", duration.Seconds()),
	})
}

// TriggerHistoricalAggregation 手动触发历史聚合
// @Summary 手动触发历史统计聚合
// @Description 批量聚合指定日期范围的统计数据，用于初次部署或补充历史数据
// @Tags Workflow
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期（格式：2006-01-02）"
// @Param end_date query string true "结束日期（格式：2006-01-02）"
// @Success 200 {object} map[string]interface{} "聚合结果"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 500 {object} core.ErrorResponse "内部错误"
// @Security BearerAuth
// @Router /workflow/stats/aggregate/historical [post]
func (ctrl *WorkflowStatsController) TriggerHistoricalAggregation(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取日期范围参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 验证日期范围（最多聚合90天）
	if endDate.Before(startDate) {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	daysDiff := int(endDate.Sub(startDate).Hours() / 24)
	if daysDiff > 90 {
		ctrl.HandleError(c, fmt.Errorf("日期范围不能超过90天"), http.StatusBadRequest)
		return
	}

	logger.Info("手动触发历史聚合",
		zap.String("start_date", startDate.Format("2006-01-02")),
		zap.String("end_date", endDate.Format("2006-01-02")),
		zap.Int("days", daysDiff+1))

	startTime := time.Now()

	// 执行批量聚合
	if err := ctrl.statsService.AggregateHistoricalStats(ctx, startDate, endDate); err != nil {
		logger.Error("历史聚合失败", zap.Error(err))
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)

	ctrl.HandleOK(c, map[string]interface{}{
		"message":    fmt.Sprintf("✅ 历史聚合完成：%s ~ %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"days":       daysDiff + 1,
		"duration":   fmt.Sprintf("%.2fs", duration.Seconds()),
	})
}
