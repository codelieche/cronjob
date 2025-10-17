// Package monitoring 提供系统监控指标收集功能
package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 系统监控指标结构体
type Metrics struct {
	// HTTP 请求相关指标
	HTTPRequestsTotal    *prometheus.CounterVec   // HTTP 请求总数
	HTTPRequestDuration  *prometheus.HistogramVec // HTTP 请求耗时
	HTTPRequestsInFlight prometheus.Gauge         // 当前正在处理的 HTTP 请求数

	// 业务相关指标
	TodoListTotal    prometheus.Gauge     // 待办事项总数
	TodoListByStatus *prometheus.GaugeVec // 按状态分组的待办事项数

	// 数据库相关指标
	DatabaseConnections prometheus.Gauge       // 数据库连接数
	DatabaseQueries     *prometheus.CounterVec // 数据库查询次数
}

// GlobalMetrics 全局监控指标实例
var GlobalMetrics *Metrics

func init() {
	GlobalMetrics = &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "todolist_http_requests_total",
				Help: "The total number of HTTP requests.",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "todolist_http_request_duration_seconds",
				Help: "The HTTP request latencies in seconds.",
			},
			[]string{"method", "endpoint"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "todolist_http_requests_in_flight",
				Help: "The current number of HTTP requests being processed.",
			},
		),
		TodoListTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "todolist_todos_total",
				Help: "The total number of todo items.",
			},
		),
		TodoListByStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "todolist_todos_by_status",
				Help: "The number of todo items by status.",
			},
			[]string{"status"},
		),
		DatabaseConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "todolist_database_connections",
				Help: "The current number of database connections.",
			},
		),
		DatabaseQueries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "todolist_database_queries_total",
				Help: "The total number of database queries.",
			},
			[]string{"operation", "table"},
		),
	}
}
