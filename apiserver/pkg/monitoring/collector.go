// Package monitoring 业务指标收集器
//
// 提供定期收集业务指标的功能
// 包括CronJob数量、任务状态统计等
package monitoring

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// BusinessMetricsCollector 业务指标收集器
type BusinessMetricsCollector struct {
	cronJobStore core.CronJobStore
	taskStore    core.TaskStore
	interval     time.Duration
	stopChan     chan struct{}
}

// NewBusinessMetricsCollector 创建业务指标收集器
func NewBusinessMetricsCollector(cronJobStore core.CronJobStore, taskStore core.TaskStore, interval time.Duration) *BusinessMetricsCollector {
	return &BusinessMetricsCollector{
		cronJobStore: cronJobStore,
		taskStore:    taskStore,
		interval:     interval,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动指标收集
func (bmc *BusinessMetricsCollector) Start(ctx context.Context) {
	logger.Info("启动业务指标收集器", zap.Duration("interval", bmc.interval))

	ticker := time.NewTicker(bmc.interval)
	defer ticker.Stop()

	// 立即收集一次指标
	bmc.collectMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("业务指标收集器已停止")
			return
		case <-bmc.stopChan:
			logger.Info("业务指标收集器收到停止信号")
			return
		case <-ticker.C:
			bmc.collectMetrics(ctx)
		}
	}
}

// Stop 停止指标收集
func (bmc *BusinessMetricsCollector) Stop() {
	close(bmc.stopChan)
}

// collectMetrics 收集业务指标
func (bmc *BusinessMetricsCollector) collectMetrics(ctx context.Context) {
	logger.Debug("开始收集业务指标")

	// 收集CronJob指标
	bmc.collectCronJobMetrics(ctx)

	// 收集Task指标
	bmc.collectTaskMetrics(ctx)

	logger.Debug("业务指标收集完成")
}

// collectCronJobMetrics 收集CronJob指标
func (bmc *BusinessMetricsCollector) collectCronJobMetrics(ctx context.Context) {
	// 统计总的CronJob数量
	totalCount, err := bmc.cronJobStore.Count(ctx)
	if err != nil {
		logger.Error("获取CronJob总数失败", zap.Error(err))
	} else {
		GlobalMetrics.CronJobsTotal.Set(float64(totalCount))
	}

	// 统计激活的CronJob数量
	activeFilter := &filters.FilterOption{
		Column: "is_active",
		Value:  true,
		Op:     filters.FILTER_EQ,
	}
	activeCount, err := bmc.cronJobStore.Count(ctx, activeFilter)
	if err != nil {
		logger.Error("获取激活CronJob数量失败", zap.Error(err))
	} else {
		GlobalMetrics.CronJobsActive.Set(float64(activeCount))
	}
}

// collectTaskMetrics 收集Task指标
func (bmc *BusinessMetricsCollector) collectTaskMetrics(ctx context.Context) {
	// 定义所有任务状态
	statuses := []string{
		core.TaskStatusPending,
		core.TaskStatusRunning,
		core.TaskStatusSuccess,
		core.TaskStatusFailed,
		core.TaskStatusError,
		core.TaskStatusTimeout,
		core.TaskStatusCanceled,
		core.TaskStatusRetrying,
	}

	// 统计每种状态的任务数量
	for _, status := range statuses {
		filter := &filters.FilterOption{
			Column: "status",
			Value:  status,
			Op:     filters.FILTER_EQ,
		}

		count, err := bmc.taskStore.Count(ctx, filter)
		if err != nil {
			logger.Error("获取任务状态统计失败",
				zap.String("status", status),
				zap.Error(err))
			continue
		}

		// 设置指标值（现在使用Gauge，可以正确设置当前状态）
		GlobalMetrics.TasksTotal.WithLabelValues(status, "default", "default").Set(float64(count))

		logger.Debug("更新任务状态指标",
			zap.String("status", status),
			zap.Int64("count", count))
	}
}

// DatabaseMetricsCollector 数据库指标收集器
type DatabaseMetricsCollector struct {
	interval time.Duration
	stopChan chan struct{}
}

// NewDatabaseMetricsCollector 创建数据库指标收集器
func NewDatabaseMetricsCollector(interval time.Duration) *DatabaseMetricsCollector {
	return &DatabaseMetricsCollector{
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start 启动数据库指标收集
func (dmc *DatabaseMetricsCollector) Start(ctx context.Context) {
	logger.Info("启动数据库指标收集器", zap.Duration("interval", dmc.interval))

	ticker := time.NewTicker(dmc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("数据库指标收集器已停止")
			return
		case <-dmc.stopChan:
			logger.Info("数据库指标收集器收到停止信号")
			return
		case <-ticker.C:
			dmc.collectDatabaseMetrics(ctx)
		}
	}
}

// Stop 停止数据库指标收集
func (dmc *DatabaseMetricsCollector) Stop() {
	close(dmc.stopChan)
}

// collectDatabaseMetrics 收集数据库指标
func (dmc *DatabaseMetricsCollector) collectDatabaseMetrics(ctx context.Context) {
	db, err := core.GetDB()
	if err != nil {
		logger.Error("获取数据库连接失败", zap.Error(err))
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("获取SQL DB失败", zap.Error(err))
		return
	}

	// 获取数据库连接池统计信息
	stats := sqlDB.Stats()

	// 更新数据库连接指标
	GlobalMetrics.DBConnections.Set(float64(stats.OpenConnections))

	logger.Debug("数据库指标收集完成",
		zap.Int("open_connections", stats.OpenConnections),
		zap.Int("in_use", stats.InUse),
		zap.Int("idle", stats.Idle))
}
