// Package middleware HTTP中间件
//
// 提供各种HTTP中间件功能，包括：
// - 监控指标收集
// - 请求日志记录
// - 性能统计等
package middleware

import (
	"strconv"
	"time"

	"github.com/codelieche/todolist/pkg/monitoring"
	"github.com/codelieche/todolist/pkg/utils/logger"
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
		monitoring.GlobalMetrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
		monitoring.GlobalMetrics.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	}
}

// MetricsCollectionMiddleware 业务指标收集中间件
// 收集业务相关的监控指标
func MetricsCollectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 处理请求
		c.Next()

		// 这里可以添加业务指标收集逻辑
		// 例如：根据请求路径和响应状态收集特定的业务指标
	}
}

// DatabaseMetricsMiddleware 数据库操作监控中间件
// 监控数据库操作的性能指标
func DatabaseMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 处理请求
		c.Next()

		// 这里可以添加数据库操作监控逻辑
		// 例如：记录数据库查询次数、耗时等
	}
}

// LoggingMiddleware 请求日志中间件
// 记录HTTP请求的详细日志
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()

		// 处理请求
		c.Next()

		// 计算请求处理时间
		duration := time.Since(start)

		// 记录请求日志
		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}
