package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/monitoring"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthController 健康检查控制器
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

// HealthStatus 健康状态枚举
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"   // 健康
	HealthStatusUnhealthy HealthStatus = "unhealthy" // 不健康
	HealthStatusDegraded  HealthStatus = "degraded"  // 降级
)

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Status    HealthStatus `json:"status"`             // 状态
	Message   string       `json:"message,omitempty"`  // 状态消息
	Timestamp time.Time    `json:"timestamp"`          // 检查时间
	Duration  string       `json:"duration,omitempty"` // 检查耗时
	Details   interface{}  `json:"details,omitempty"`  // 详细信息
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status     HealthStatus                `json:"status"`     // 整体状态
	Timestamp  time.Time                   `json:"timestamp"`  // 检查时间
	Version    string                      `json:"version"`    // 应用版本
	Uptime     string                      `json:"uptime"`     // 运行时间
	Components map[string]*ComponentHealth `json:"components"` // 各组件状态
}

// 应用启动时间
var startTime = time.Now()

// Health 健康检查接口
// @Summary 健康检查
// @Description 检查应用及其依赖服务的健康状态
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "健康状态正常"
// @Failure 503 {object} HealthResponse "服务不可用"
// @Router /health [get]
func (controller *HealthController) Health(c *gin.Context) {
	checkStartTime := time.Now()

	// 创建健康检查响应
	response := &HealthResponse{
		Status:     HealthStatusHealthy,
		Timestamp:  checkStartTime,
		Version:    "1.0.0",
		Uptime:     time.Since(startTime).String(),
		Components: make(map[string]*ComponentHealth),
	}

	// 检查数据库连接
	dbHealth := controller.checkDatabase()
	response.Components["database"] = dbHealth

	// 检查Redis连接
	redisHealth := controller.checkRedis()
	response.Components["redis"] = redisHealth

	// 检查WebSocket服务
	websocketHealth := controller.checkWebSocket()
	response.Components["websocket"] = websocketHealth

	// 检查任务队列
	taskQueueHealth := controller.checkTaskQueue()
	response.Components["task_queue"] = taskQueueHealth

	// 检查应用自身状态
	appHealth := controller.checkApplication()
	response.Components["application"] = appHealth

	// 计算整体健康状态
	response.Status = controller.calculateOverallStatus(response.Components)

	// 根据整体状态返回相应的HTTP状态码
	statusCode := http.StatusOK
	if response.Status == HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if response.Status == HealthStatusDegraded {
		statusCode = http.StatusOK // 降级状态仍返回200，但在响应中标明
	}

	c.JSON(statusCode, response)
}

// Readiness 就绪检查接口
// @Summary 就绪检查
// @Description 检查应用是否准备好接收流量
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "应用就绪"
// @Failure 503 {object} map[string]interface{} "应用未就绪"
// @Router /readiness [get]
func (controller *HealthController) Readiness(c *gin.Context) {
	// 检查关键依赖是否就绪
	dbHealth := controller.checkDatabase()

	// 数据库必须健康才算就绪
	if dbHealth.Status != HealthStatusHealthy {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "not_ready",
			"message":   "数据库连接不可用",
			"timestamp": time.Now(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"message":   "应用已就绪",
		"timestamp": time.Now(),
	})
}

// Liveness 存活检查接口
// @Summary 存活检查
// @Description 检查应用是否存活
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "应用存活"
// @Router /liveness [get]
func (controller *HealthController) Liveness(c *gin.Context) {
	// 简单的存活检查，只要能响应就说明应用存活
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"message":   "应用正在运行",
		"timestamp": time.Now(),
		"uptime":    time.Since(startTime).String(),
	})
}

// checkDatabase 检查数据库连接健康状态
func (controller *HealthController) checkDatabase() *ComponentHealth {
	start := time.Now()

	db, err := core.GetDB()
	if err != nil {
		return &ComponentHealth{
			Status:    HealthStatusUnhealthy,
			Message:   "数据库连接获取失败: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		}
	}

	// 执行简单的ping测试
	sqlDB, err := db.DB()
	if err != nil {
		return &ComponentHealth{
			Status:    HealthStatusUnhealthy,
			Message:   "获取SQL DB实例失败: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		}
	}

	// 设置ping超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return &ComponentHealth{
			Status:    HealthStatusUnhealthy,
			Message:   "数据库ping失败: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		}
	}

	// 获取连接池统计信息
	stats := sqlDB.Stats()

	return &ComponentHealth{
		Status:    HealthStatusHealthy,
		Message:   "数据库连接正常",
		Timestamp: time.Now(),
		Duration:  time.Since(start).String(),
		Details: map[string]interface{}{
			"max_open_connections": stats.MaxOpenConnections,
			"open_connections":     stats.OpenConnections,
			"in_use":               stats.InUse,
			"idle":                 stats.Idle,
		},
	}
}

// checkRedis 检查Redis连接健康状态
func (controller *HealthController) checkRedis() *ComponentHealth {
	start := time.Now()

	// 获取Redis客户端
	redisClient, err := core.GetRedis()
	if err != nil {
		return &ComponentHealth{
			Status:    HealthStatusDegraded, // Redis不可用时标记为降级，而不是不健康
			Message:   "Redis连接获取失败: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		}
	}

	// 设置ping超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 执行ping测试
	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return &ComponentHealth{
			Status:    HealthStatusDegraded, // Redis不可用时标记为降级，而不是不健康
			Message:   "Redis ping失败: " + err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start).String(),
		}
	}

	// 获取Redis信息
	_, err = redisClient.Info(ctx, "server").Result()
	var redisVersion string
	if err == nil {
		// 简单解析版本信息（这里可以更复杂的解析）
		redisVersion = "connected"
	}

	return &ComponentHealth{
		Status:    HealthStatusHealthy,
		Message:   "Redis连接正常: " + pong,
		Timestamp: time.Now(),
		Duration:  time.Since(start).String(),
		Details: map[string]interface{}{
			"version": redisVersion,
		},
	}
}

// checkWebSocket 检查WebSocket服务健康状态
func (controller *HealthController) checkWebSocket() *ComponentHealth {
	start := time.Now()

	// 获取WebSocket客户端数量
	websocketClientsCount := 0
	workers := make(map[string]*core.Worker)

	if controller.websocketService != nil {
		clientManager := controller.websocketService.GetClientManager()
		if clientManager != nil {
			websocketClientsCount = clientManager.Count()
			workers = clientManager.GetWorkers()

			// 更新监控指标
			monitoring.GlobalMetrics.WebSocketConnections.Set(float64(websocketClientsCount))
			monitoring.GlobalMetrics.UpdateWorkerCount(websocketClientsCount)
		}
	}

	return &ComponentHealth{
		Status:    HealthStatusHealthy,
		Message:   "WebSocket服务正常",
		Timestamp: time.Now(),
		Duration:  time.Since(start).String(),
		Details: map[string]interface{}{
			"connected_clients": websocketClientsCount,
			"workers":           workers,
		},
	}
}

// checkTaskQueue 检查任务队列健康状态
func (controller *HealthController) checkTaskQueue() *ComponentHealth {
	start := time.Now()
	ctx := context.Background()

	// 获取Pending任务数量
	pendingTasksCount := 0
	if controller.taskService != nil {
		// 使用过滤器获取状态为Pending的任务数量
		filter := &filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		}

		count, err := controller.taskService.Count(ctx, filter)
		if err != nil {
			logger.Error("获取Pending任务数量失败", zap.Error(err))
			return &ComponentHealth{
				Status:    HealthStatusDegraded,
				Message:   "任务队列状态获取失败: " + err.Error(),
				Timestamp: time.Now(),
				Duration:  time.Since(start).String(),
			}
		} else {
			pendingTasksCount = int(count)
			// 更新任务队列大小指标
			monitoring.GlobalMetrics.TaskQueueSize.Set(float64(pendingTasksCount))
		}
	}

	return &ComponentHealth{
		Status:    HealthStatusHealthy,
		Message:   "任务队列正常",
		Timestamp: time.Now(),
		Duration:  time.Since(start).String(),
		Details: map[string]interface{}{
			"pending_tasks": pendingTasksCount,
		},
	}
}

// checkApplication 检查应用自身状态
func (controller *HealthController) checkApplication() *ComponentHealth {
	start := time.Now()

	return &ComponentHealth{
		Status:    HealthStatusHealthy,
		Message:   "应用运行正常",
		Timestamp: time.Now(),
		Duration:  time.Since(start).String(),
		Details: map[string]interface{}{
			"uptime":     time.Since(startTime).String(),
			"start_time": startTime,
			"version":    "1.0.0",
		},
	}
}

// calculateOverallStatus 计算整体健康状态
func (controller *HealthController) calculateOverallStatus(components map[string]*ComponentHealth) HealthStatus {
	hasUnhealthy := false
	hasDegraded := false

	for name, component := range components {
		switch component.Status {
		case HealthStatusUnhealthy:
			// 数据库不健康时，整体状态为不健康
			if name == "database" {
				return HealthStatusUnhealthy
			}
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	// 如果有不健康的组件（非数据库），标记为降级
	if hasUnhealthy {
		return HealthStatusDegraded
	}

	// 如果有降级的组件，标记为降级
	if hasDegraded {
		return HealthStatusDegraded
	}

	return HealthStatusHealthy
}
