// Package monitoring 监控指标定义和收集
//
// 提供Prometheus监控指标的定义、注册和收集功能
// 包括系统指标、业务指标和自定义指标
package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector 监控指标收集器
type MetricsCollector struct {
	// ========== 系统基础指标 ==========

	// HTTP请求相关指标
	HTTPRequestsTotal    *prometheus.CounterVec   // HTTP请求总数
	HTTPRequestDuration  *prometheus.HistogramVec // HTTP请求响应时间
	HTTPRequestsInFlight prometheus.Gauge         // 当前正在处理的HTTP请求数

	// ========== 业务核心指标 ==========

	// CronJob相关指标
	CronJobsTotal            prometheus.Gauge         // CronJob总数
	CronJobsActive           prometheus.Gauge         // 激活的CronJob数量
	CronJobExecutions        *prometheus.CounterVec   // CronJob执行次数
	CronJobExecutionDuration *prometheus.HistogramVec // CronJob执行时长

	// Task相关指标
	TasksTotal            *prometheus.GaugeVec     // 任务总数（按状态分类）- 使用Gauge反映当前状态
	TaskExecutions        *prometheus.CounterVec   // 任务执行次数
	TaskExecutionDuration *prometheus.HistogramVec // 任务执行时长
	TaskQueueSize         prometheus.Gauge         // 任务队列大小
	TaskErrors            *prometheus.CounterVec   // 任务错误数

	// Worker相关指标
	WorkersConnected     prometheus.Gauge       // 连接的Worker数量
	WorkerConnections    *prometheus.CounterVec // Worker连接事件
	WorkerTasksProcessed *prometheus.CounterVec // Worker处理的任务数

	// ========== 基础设施指标 ==========

	// 数据库相关指标
	DBConnections   prometheus.Gauge         // 数据库连接数
	DBQueryDuration *prometheus.HistogramVec // 数据库查询时长
	DBErrors        *prometheus.CounterVec   // 数据库错误数

	// Redis相关指标
	RedisConnections     prometheus.Gauge         // Redis连接数
	RedisCommandDuration *prometheus.HistogramVec // Redis命令执行时长
	RedisErrors          *prometheus.CounterVec   // Redis错误数

	// WebSocket相关指标
	WebSocketConnections prometheus.Gauge       // WebSocket连接数
	WebSocketMessages    *prometheus.CounterVec // WebSocket消息数
	WebSocketErrors      *prometheus.CounterVec // WebSocket错误数

	// ========== 分布式锁指标 ==========

	LockAcquisitions *prometheus.CounterVec   // 锁获取次数
	LockDuration     *prometheus.HistogramVec // 锁持有时长
	LockErrors       *prometheus.CounterVec   // 锁操作错误数
}

// NewMetricsCollector 创建监控指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		// HTTP请求指标
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_http_requests_in_flight",
				Help: "Current number of HTTP requests being processed",
			},
		),

		// CronJob指标
		CronJobsTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_cronjobs_total",
				Help: "Total number of CronJobs",
			},
		),
		CronJobsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_cronjobs_active",
				Help: "Number of active CronJobs",
			},
		),
		CronJobExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_cronjob_executions_total",
				Help: "Total number of CronJob executions",
			},
			[]string{"cronjob_id", "project", "category"},
		),
		CronJobExecutionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_cronjob_execution_duration_seconds",
				Help:    "CronJob execution duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 600, 1800, 3600},
			},
			[]string{"cronjob_id", "project", "category"},
		),

		// Task指标
		TasksTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cronjob_tasks_total",
				Help: "Current number of tasks by status",
			},
			[]string{"status", "project", "category"},
		),
		TaskExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_task_executions_total",
				Help: "Total number of task executions",
			},
			[]string{"task_id", "worker_id", "status"},
		),
		TaskExecutionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_task_execution_duration_seconds",
				Help:    "Task execution duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 600, 1800, 3600},
			},
			[]string{"category", "worker_id"},
		),
		TaskQueueSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_task_queue_size",
				Help: "Current size of task queue",
			},
		),
		TaskErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_task_errors_total",
				Help: "Total number of task errors",
			},
			[]string{"error_type", "category", "worker_id"},
		),

		// Worker指标
		WorkersConnected: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_workers_connected",
				Help: "Number of connected workers",
			},
		),
		WorkerConnections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_worker_connections_total",
				Help: "Total number of worker connection events",
			},
			[]string{"worker_id", "event_type"}, // event_type: connected, disconnected
		),
		WorkerTasksProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_worker_tasks_processed_total",
				Help: "Total number of tasks processed by workers",
			},
			[]string{"worker_id", "worker_name", "status"},
		),

		// 数据库指标
		DBConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_db_connections",
				Help: "Number of database connections",
			},
		),
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5},
			},
			[]string{"operation", "table"},
		),
		DBErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_db_errors_total",
				Help: "Total number of database errors",
			},
			[]string{"operation", "error_type"},
		),

		// Redis指标
		RedisConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_redis_connections",
				Help: "Number of Redis connections",
			},
		),
		RedisCommandDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_redis_command_duration_seconds",
				Help:    "Redis command duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2},
			},
			[]string{"command"},
		),
		RedisErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_redis_errors_total",
				Help: "Total number of Redis errors",
			},
			[]string{"command", "error_type"},
		),

		// WebSocket指标
		WebSocketConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "cronjob_websocket_connections",
				Help: "Number of WebSocket connections",
			},
		),
		WebSocketMessages: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_websocket_messages_total",
				Help: "Total number of WebSocket messages",
			},
			[]string{"direction", "message_type"}, // direction: sent, received
		),
		WebSocketErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_websocket_errors_total",
				Help: "Total number of WebSocket errors",
			},
			[]string{"error_type"},
		),

		// 分布式锁指标
		LockAcquisitions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_lock_acquisitions_total",
				Help: "Total number of lock acquisitions",
			},
			[]string{"lock_key", "result"}, // result: success, failure
		),
		LockDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cronjob_lock_duration_seconds",
				Help:    "Lock hold duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 600},
			},
			[]string{"lock_key"},
		),
		LockErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cronjob_lock_errors_total",
				Help: "Total number of lock operation errors",
			},
			[]string{"operation", "error_type"},
		),
	}
}

// RecordHTTPRequest 记录HTTP请求指标
func (mc *MetricsCollector) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration) {
	mc.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	mc.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordTaskExecution 记录任务执行指标
func (mc *MetricsCollector) RecordTaskExecution(taskID, workerID, status string, duration time.Duration, category string) {
	mc.TaskExecutions.WithLabelValues(taskID, workerID, status).Inc()
	mc.TaskExecutionDuration.WithLabelValues(category, workerID).Observe(duration.Seconds())
}

// RecordTaskError 记录任务错误指标
func (mc *MetricsCollector) RecordTaskError(errorType, category, workerID string) {
	mc.TaskErrors.WithLabelValues(errorType, category, workerID).Inc()
}

// UpdateWorkerCount 更新Worker连接数
func (mc *MetricsCollector) UpdateWorkerCount(count int) {
	mc.WorkersConnected.Set(float64(count))
}

// RecordWorkerConnection 记录Worker连接事件
func (mc *MetricsCollector) RecordWorkerConnection(workerID, eventType string) {
	mc.WorkerConnections.WithLabelValues(workerID, eventType).Inc()
}

// RecordDBQuery 记录数据库查询指标
func (mc *MetricsCollector) RecordDBQuery(operation, table string, duration time.Duration) {
	mc.DBQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// RecordDBError 记录数据库错误指标
func (mc *MetricsCollector) RecordDBError(operation, errorType string) {
	mc.DBErrors.WithLabelValues(operation, errorType).Inc()
}

// RecordLockOperation 记录分布式锁操作指标
func (mc *MetricsCollector) RecordLockOperation(lockKey, result string, duration time.Duration) {
	mc.LockAcquisitions.WithLabelValues(lockKey, result).Inc()
	if result == "success" {
		mc.LockDuration.WithLabelValues(lockKey).Observe(duration.Seconds())
	}
}

// 全局监控指标收集器实例
var GlobalMetrics = NewMetricsCollector()
