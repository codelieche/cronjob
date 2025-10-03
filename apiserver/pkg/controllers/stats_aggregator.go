package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StatsAggregatorController 统计数据聚合控制器
//
// 提供手动触发统计数据聚合的 API 接口
// 用于服务挂掉后的数据补偿
type StatsAggregatorController struct {
	controllers.BaseController
	aggregator *services.StatsAggregator
	locker     core.Locker
}

// NewStatsAggregatorController 创建统计数据聚合控制器实例
func NewStatsAggregatorController(aggregator *services.StatsAggregator, locker core.Locker) *StatsAggregatorController {
	return &StatsAggregatorController{
		aggregator: aggregator,
		locker:     locker,
	}
}

// TriggerDailyAggregation 手动触发每日统计数据聚合
//
// @Summary 手动触发每日统计数据聚合
// @Description 手动触发统计数据聚合任务，用于服务挂掉后的数据补偿。使用分布式锁防止并发执行。
// @Tags Stats
// @Accept json
// @Produce json
// @Param date query string false "聚合日期（格式：2006-01-02，默认为昨天）"
// @Success 200 {object} map[string]interface{} "聚合成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 403 {object} core.ErrorResponse "权限不足（需要管理员权限）"
// @Failure 409 {object} core.ErrorResponse "聚合任务正在执行中"
// @Failure 500 {object} core.ErrorResponse "聚合失败"
// @Security BearerAuth
// @Router /stats/aggregate/daily [post]
func (ctrl *StatsAggregatorController) TriggerDailyAggregation(c *gin.Context) {
	ctx := context.Background()
	lockKey := "stats:aggregator:manual"

	// 获取日期参数（可选）
	dateParam := c.Query("date")
	if dateParam == "" {
		// 默认为昨天
		dateParam = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	// 🔥 尝试获取锁（5分钟过期）
	lock, err := ctrl.locker.TryAcquire(ctx, lockKey, 5*time.Minute)
	if err != nil {
		if err == core.ErrLockAlreadyAcquired {
			logger.Warn("统计数据聚合任务正在执行中", zap.String("lock_key", lockKey))
			c.JSON(http.StatusConflict, gin.H{
				"code":    http.StatusConflict,
				"message": "聚合任务正在执行中，请稍后再试",
			})
			return
		}
		logger.Error("获取聚合任务锁失败", zap.String("lock_key", lockKey), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "获取锁失败",
		})
		return
	}
	defer lock.Release(ctx)

	logger.Info("开始手动触发统计数据聚合",
		zap.String("date", dateParam),
		zap.String("triggered_by", ctrl.getUsernameFromContext(c)),
		zap.String("lock_key", lockKey))

	startTime := time.Now()

	// 执行聚合
	if err := ctrl.aggregator.AggregateDailyStats(dateParam); err != nil {
		logger.Error("统计数据聚合失败",
			zap.String("date", dateParam),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "聚合失败: " + err.Error(),
		})
		return
	}

	// 🔥 聚合成功后清除Redis缓存，确保用户立即看到最新数据
	ctrl.clearStatsCache()

	duration := time.Since(startTime)
	logger.Info("统计数据聚合成功",
		zap.String("date", dateParam),
		zap.Duration("duration", duration),
		zap.String("triggered_by", ctrl.getUsernameFromContext(c)))

	ctrl.HandleOK(c, gin.H{
		"message":  "聚合成功",
		"date":     dateParam,
		"duration": duration.String(),
	})
}

// TriggerHistoricalAggregation 手动触发历史数据聚合
//
// @Summary 手动触发历史数据聚合
// @Description 批量聚合指定日期范围的历史数据，用于初次部署或数据迁移。使用分布式锁防止并发执行。
// @Tags Stats
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期（格式：2006-01-02）"
// @Param end_date query string true "结束日期（格式：2006-01-02）"
// @Success 200 {object} map[string]interface{} "聚合成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 403 {object} core.ErrorResponse "权限不足（需要管理员权限）"
// @Failure 409 {object} core.ErrorResponse "聚合任务正在执行中"
// @Failure 500 {object} core.ErrorResponse "聚合失败"
// @Security BearerAuth
// @Router /stats/aggregate/historical [post]
func (ctrl *StatsAggregatorController) TriggerHistoricalAggregation(c *gin.Context) {
	ctx := context.Background()
	lockKey := "stats:aggregator:historical"

	// 获取日期参数
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "start_date 和 end_date 参数必填",
		})
		return
	}

	// 验证日期格式
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "start_date 格式错误，应为 2006-01-02",
		})
		return
	}
	if _, err := time.Parse("2006-01-02", endDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "end_date 格式错误，应为 2006-01-02",
		})
		return
	}

	// 🔥 尝试获取锁（30分钟过期，历史数据可能较多）
	lock, err := ctrl.locker.TryAcquire(ctx, lockKey, 30*time.Minute)
	if err != nil {
		if err == core.ErrLockAlreadyAcquired {
			logger.Warn("历史数据聚合任务正在执行中", zap.String("lock_key", lockKey))
			c.JSON(http.StatusConflict, gin.H{
				"code":    http.StatusConflict,
				"message": "历史数据聚合任务正在执行中，请稍后再试",
			})
			return
		}
		logger.Error("获取聚合任务锁失败", zap.String("lock_key", lockKey), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "获取锁失败",
		})
		return
	}
	defer lock.Release(ctx)

	logger.Info("开始手动触发历史数据聚合",
		zap.String("start_date", startDate),
		zap.String("end_date", endDate),
		zap.String("triggered_by", ctrl.getUsernameFromContext(c)),
		zap.String("lock_key", lockKey))

	startTime := time.Now()

	// 执行历史数据聚合
	if err := ctrl.aggregator.AggregateHistoricalStats(startDate, endDate); err != nil {
		logger.Error("历史数据聚合失败",
			zap.String("start_date", startDate),
			zap.String("end_date", endDate),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "聚合失败: " + err.Error(),
		})
		return
	}

	// 🔥 聚合成功后清除Redis缓存，确保用户立即看到最新数据
	ctrl.clearStatsCache()

	duration := time.Since(startTime)
	logger.Info("历史数据聚合成功",
		zap.String("start_date", startDate),
		zap.String("end_date", endDate),
		zap.Duration("duration", duration),
		zap.String("triggered_by", ctrl.getUsernameFromContext(c)))

	ctrl.HandleOK(c, gin.H{
		"message":    "聚合成功",
		"start_date": startDate,
		"end_date":   endDate,
		"duration":   duration.String(),
	})
}

// clearStatsCache 清除统计分析的Redis缓存
//
// 在手动触发聚合后调用，确保用户立即看到最新数据
// 使用模式匹配删除所有 stats:analysis:* 键
func (ctrl *StatsAggregatorController) clearStatsCache() {
	ctx := context.Background()
	redis, err := core.GetRedis()
	if err != nil {
		logger.Warn("获取Redis连接失败，跳过缓存清除", zap.Error(err))
		return
	}

	// 使用SCAN命令遍历删除（比KEYS命令更安全，不会阻塞Redis）
	pattern := "stats:analysis:*"
	iter := redis.Scan(ctx, 0, pattern, 100).Iterator()

	deletedCount := 0
	for iter.Next(ctx) {
		key := iter.Val()
		if err := redis.Del(ctx, key).Err(); err != nil {
			logger.Warn("删除缓存键失败", zap.String("key", key), zap.Error(err))
		} else {
			deletedCount++
		}
	}

	if err := iter.Err(); err != nil {
		logger.Error("扫描缓存键失败", zap.Error(err))
	} else if deletedCount > 0 {
		logger.Info("已清除统计分析缓存",
			zap.Int("deleted_count", deletedCount),
			zap.String("pattern", pattern))
	}
}

// getUsernameFromContext 从上下文中获取用户名
func (ctrl *StatsAggregatorController) getUsernameFromContext(c *gin.Context) string {
	if user, exists := c.Get(core.ContextKeyUsername); exists {
		if username, ok := user.(string); ok && username != "" {
			return username
		}
	}
	return "unknown"
}
