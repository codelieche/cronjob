// Package middleware HTTP中间件
//
// 提供各种HTTP中间件功能，包括：
// - 监控指标收集
// - 请求日志记录
// - 性能统计等
package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/monitoring"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PrometheusMiddleware Prometheus监控中间件
// 自动收集HTTP请求的监控指标
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()

		// 增加正在处理的请求计数
		monitoring.GlobalMetrics.HTTPRequestsInFlight.Inc()

		// 处理请求
		c.Next()

		// 减少正在处理的请求计数
		monitoring.GlobalMetrics.HTTPRequestsInFlight.Dec()

		// 计算请求处理时间
		duration := time.Since(start)

		// 获取请求信息
		method := c.Request.Method
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}
		statusCode := strconv.Itoa(c.Writer.Status())

		// 记录监控指标
		monitoring.GlobalMetrics.RecordHTTPRequest(method, endpoint, statusCode, duration)
	}
}

// MetricsCollectionMiddleware 指标收集中间件
// 提供更详细的指标收集功能，包括业务指标和性能分析
func MetricsCollectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 收集详细指标
		duration := time.Since(start)
		method := c.Request.Method
		endpoint := c.FullPath()
		statusCode := c.Writer.Status()

		// 根据不同的端点收集特定的业务指标
		collectBusinessMetrics(c, method, endpoint, statusCode, duration)

		// 记录慢请求（超过1秒的请求）
		if duration > time.Second {
			logger.Warn("检测到慢请求",
				zap.String("method", method),
				zap.String("endpoint", endpoint),
				zap.Duration("duration", duration),
				zap.Int("status_code", statusCode),
				zap.String("user_agent", c.GetHeader("User-Agent")),
				zap.String("remote_addr", c.ClientIP()))

			// 记录慢请求指标（可以添加到监控指标中）
			monitoring.GlobalMetrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
		}

		// 记录错误请求的详细信息
		if statusCode >= 400 {
			collectErrorMetrics(c, method, endpoint, statusCode, duration)
		}
	}
}

// collectBusinessMetrics 收集业务相关的指标
func collectBusinessMetrics(c *gin.Context, method, endpoint string, statusCode int, duration time.Duration) {
	switch {
	// ========== CronJob相关指标 ==========
	case strings.HasPrefix(endpoint, "/api/v1/cronjob"):
		if method == "POST" && statusCode >= 200 && statusCode < 300 {
			// CronJob创建成功指标
			monitoring.GlobalMetrics.CronJobExecutions.WithLabelValues(
				"unknown", // cronjob_id 在这里无法获取，可以从请求体中解析
				getProjectFromContext(c),
				getCategoryFromContext(c),
			).Inc()
		}

	// ========== Task相关指标 ==========
	case strings.HasPrefix(endpoint, "/api/v1/task"):
		switch method {
		case "POST":
			if statusCode >= 200 && statusCode < 300 {
				// 任务创建成功指标
				monitoring.GlobalMetrics.TaskExecutions.WithLabelValues(
					"unknown", // task_id
					"unknown", // worker_id
					"created", // status
				).Inc()
			}
		case "PUT":
			// 任务更新指标
			if strings.Contains(endpoint, "update-status") && statusCode >= 200 && statusCode < 300 {
				// 任务状态更新指标
				monitoring.GlobalMetrics.TaskExecutions.WithLabelValues(
					extractIDFromPath(c.Param("id")),
					"unknown", // worker_id
					"updated", // status
				).Inc()
			}
		}

	// ========== Worker相关指标 ==========
	case strings.HasPrefix(endpoint, "/api/v1/worker"):
		if method == "POST" && statusCode >= 200 && statusCode < 300 {
			// Worker注册指标
			monitoring.GlobalMetrics.WorkerConnections.WithLabelValues(
				extractIDFromPath(c.Param("id")),
				"registered",
			).Inc()
		}
		if strings.Contains(endpoint, "ping") && statusCode >= 200 && statusCode < 300 {
			// Worker心跳指标
			monitoring.GlobalMetrics.WorkerConnections.WithLabelValues(
				extractIDFromPath(c.Param("id")),
				"heartbeat",
			).Inc()
		}

	// ========== WebSocket相关指标 ==========
	case strings.HasPrefix(endpoint, "/api/v1/ws"):
		if statusCode == 101 { // WebSocket升级成功
			monitoring.GlobalMetrics.WebSocketConnections.Inc()
			monitoring.GlobalMetrics.WebSocketMessages.WithLabelValues(
				"connection",
				"upgrade",
			).Inc()
		}

	// ========== 健康检查指标 ==========
	case endpoint == "/api/v1/health/":
		// 健康检查响应时间指标
		monitoring.GlobalMetrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())

		if statusCode >= 200 && statusCode < 300 {
			// 系统健康状态正常
		} else {
			// 系统健康状态异常，可能需要告警
			logger.Error("健康检查失败",
				zap.Int("status_code", statusCode),
				zap.Duration("duration", duration))
		}

	// ========== 分布式锁相关指标 ==========
	case strings.HasPrefix(endpoint, "/api/v1/lock"):
		switch {
		case strings.Contains(endpoint, "acquire"):
			if statusCode >= 200 && statusCode < 300 {
				monitoring.GlobalMetrics.LockAcquisitions.WithLabelValues(
					c.Query("key"), // 从查询参数获取锁键名
					"success",
				).Inc()
			} else {
				monitoring.GlobalMetrics.LockAcquisitions.WithLabelValues(
					c.Query("key"),
					"failure",
				).Inc()
			}
		case strings.Contains(endpoint, "release"):
			// 锁释放指标
			monitoring.GlobalMetrics.LockAcquisitions.WithLabelValues(
				c.Query("key"),
				"released",
			).Inc()
		}
	}
}

// collectErrorMetrics 收集错误相关的指标
func collectErrorMetrics(c *gin.Context, method, endpoint string, statusCode int, duration time.Duration) {
	// 根据错误类型分类
	var errorType string
	switch {
	case statusCode >= 400 && statusCode < 500:
		errorType = "client_error"
	case statusCode >= 500:
		errorType = "server_error"
	default:
		errorType = "unknown_error"
	}

	// 记录详细的错误日志
	logger.Error("HTTP请求错误",
		zap.String("method", method),
		zap.String("endpoint", endpoint),
		zap.Int("status_code", statusCode),
		zap.Duration("duration", duration),
		zap.String("error_type", errorType),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("remote_addr", c.ClientIP()),
		zap.String("request_id", c.GetHeader("X-Request-ID")))

	// 根据不同的业务端点记录特定的错误指标
	switch {
	case strings.HasPrefix(endpoint, "/api/v1/task"):
		monitoring.GlobalMetrics.TaskErrors.WithLabelValues(
			errorType,
			getCategoryFromContext(c),
			"unknown", // worker_id
		).Inc()
	case strings.HasPrefix(endpoint, "/api/v1/cronjob"):
		// CronJob相关错误指标 - 可以根据需要添加特定的CronJob错误监控
		logger.Debug("CronJob相关请求错误", zap.String("endpoint", endpoint))
	case strings.HasPrefix(endpoint, "/api/v1/worker"):
		// Worker相关错误指标 - 可以根据需要添加特定的Worker错误监控
		logger.Debug("Worker相关请求错误", zap.String("endpoint", endpoint))
	}
}

// 辅助函数：从上下文中获取项目信息
func getProjectFromContext(c *gin.Context) string {
	if project := c.Query("project"); project != "" {
		return project
	}
	if project := c.GetHeader("X-Project"); project != "" {
		return project
	}
	return "default"
}

// 辅助函数：从上下文中获取分类信息
func getCategoryFromContext(c *gin.Context) string {
	if category := c.Query("category"); category != "" {
		return category
	}
	if category := c.GetHeader("X-Category"); category != "" {
		return category
	}
	return "default"
}

// 辅助函数：从路径中提取ID（简单实现）
func extractIDFromPath(id string) string {
	if id == "" {
		return "unknown"
	}
	return id
}

// DatabaseMetricsMiddleware 数据库操作监控中间件
// 记录数据库相关的操作指标
func DatabaseMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		duration := time.Since(start)
		endpoint := c.FullPath()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// 推断数据库操作类型
		var operation string
		switch method {
		case "GET":
			operation = "select"
		case "POST":
			operation = "insert"
		case "PUT", "PATCH":
			operation = "update"
		case "DELETE":
			operation = "delete"
		default:
			operation = "unknown"
		}

		// 推断操作的表名
		table := inferTableFromEndpoint(endpoint)

		// 记录数据库操作指标
		monitoring.GlobalMetrics.RecordDBQuery(operation, table, duration)

		// 如果操作失败，记录错误指标
		if statusCode >= 400 {
			var errorType string
			if statusCode >= 500 {
				errorType = "server_error"
			} else {
				errorType = "client_error"
			}
			monitoring.GlobalMetrics.RecordDBError(operation, errorType)
		}
	}
}

// 辅助函数：从端点推断数据库表名
func inferTableFromEndpoint(endpoint string) string {
	switch {
	case strings.Contains(endpoint, "cronjob"):
		return "cronjobs"
	case strings.Contains(endpoint, "task"):
		return "tasks"
	case strings.Contains(endpoint, "worker"):
		return "workers"
	case strings.Contains(endpoint, "category"):
		return "categories"
	case strings.Contains(endpoint, "user"):
		return "users"
	case strings.Contains(endpoint, "tasklog"):
		return "task_logs"
	default:
		return "unknown"
	}
}
