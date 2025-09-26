package controllers

import (
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/monitoring"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type HealthController struct {
	controllers.BaseController
	websocketService core.WebsocketService // WebSocket服务
	taskService      core.TaskService      // 任务服务
}

// NewHealthController 创建HealthController实例
func NewHealthController(
	websocketService core.WebsocketService,
	taskService core.TaskService,
) *HealthController {
	return &HealthController{
		websocketService: websocketService,
		taskService:      taskService,
	}
}

// Health 健康检查接口
// @Summary 系统健康检查
// @Description 检查系统各个组件的健康状态，包括MySQL、Redis、WebSocket连接数和待处理任务数
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "系统健康状态信息"
// @Failure 503 {object} map[string]interface{} "服务降级或不可用"
// @Router /health/ [get]
func (hc *HealthController) Health(c *gin.Context) {
	// 创建上下文
	ctx := c.Request.Context()

	// 检查MySQL连接
	dbStatus := "ok"
	dbCheckTime := time.Now()
	db, err := core.GetDB()
	if err != nil {
		dbStatus = "error"
		logger.Error("MySQL连接失败", zap.Error(err))
	} else {
		// 尝试Ping数据库
		sqlDB, err := db.DB()
		if err != nil {
			dbStatus = "error"
			logger.Error("获取SQL DB失败", zap.Error(err))
		} else {
			if err := sqlDB.Ping(); err != nil {
				dbStatus = "error"
				logger.Error("MySQL Ping失败", zap.Error(err))
			}
		}
	}
	dbCheckDuration := time.Since(dbCheckTime)

	// 检查Redis连接
	redisStatus := "ok"
	redisCheckTime := time.Now()
	redisClient, err := core.GetRedis()
	if err != nil {
		redisStatus = "error"
		logger.Error("Redis连接失败", zap.Error(err))
	} else {
		// 尝试Ping Redis
		if _, err := redisClient.Ping(ctx).Result(); err != nil {
			redisStatus = "error"
			logger.Error("Redis Ping失败", zap.Error(err))
		}
	}
	redisCheckDuration := time.Since(redisCheckTime)

	// 获取WebSocket客户端数量
	websocketClientsCount := 0
	workers := make(map[string]*core.Worker)
	if hc.websocketService != nil {
		clientManager := hc.websocketService.GetClientManager()
		if clientManager != nil {
			websocketClientsCount = clientManager.Count()
			workers = clientManager.GetWorkers()

			// 更新监控指标
			monitoring.GlobalMetrics.WebSocketConnections.Set(float64(websocketClientsCount))
			monitoring.GlobalMetrics.UpdateWorkerCount(websocketClientsCount)
		}
	}

	// 获取Pending任务数量
	pendingTasksCount := 0
	if hc.taskService != nil {
		// 使用过滤器获取状态为Pending的任务数量
		filter := &filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		}

		count, err := hc.taskService.Count(ctx, filter)
		if err != nil {
			logger.Error("获取Pending任务数量失败", zap.Error(err))
		} else {
			pendingTasksCount = int(count)
			// 更新任务队列大小指标
			monitoring.GlobalMetrics.TaskQueueSize.Set(float64(pendingTasksCount))
		}
	}

	// 构建响应
	response := gin.H{
		"status": "ok",
		"services": gin.H{
			"mysql": gin.H{
				"status":    dbStatus,
				"latency":   dbCheckDuration.String(),
				"timestamp": dbCheckTime.Format(time.RFC3339),
			},
			"redis": gin.H{
				"status":    redisStatus,
				"latency":   redisCheckDuration.String(),
				"timestamp": redisCheckTime.Format(time.RFC3339),
			},
		},
		"workers": workers,
		"metrics": gin.H{
			"websocket_clients": websocketClientsCount,
			"pending_tasks":     pendingTasksCount,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// 如果有任一服务异常，更新整体状态
	if dbStatus != "ok" || redisStatus != "ok" {
		response["status"] = "degraded"
	}

	hc.HandleOK(c, response)
}
