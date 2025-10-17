// Package monitoring 提供系统监控指标收集功能
package monitoring

import (
	"context"
	"time"

	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"go.uber.org/zap"
)

// UpdateTodoMetrics 更新待办事项相关的监控指标
func UpdateTodoMetrics(ctx context.Context, todoService core.TodoListService) {
	if todoService == nil {
		return
	}

	// 获取待办事项总数
	total, err := todoService.Count(ctx)
	if err != nil {
		logger.Error("failed to get total todos for metrics", zap.Error(err))
		return
	}
	GlobalMetrics.TodoListTotal.Set(float64(total))

	// 获取各状态的待办事项数量
	statuses := []string{
		core.TodoStatusPending,
		core.TodoStatusRunning,
		core.TodoStatusDone,
		core.TodoStatusCanceled,
	}

	for _, status := range statuses {
		// 这里需要实现状态过滤，简化处理
		GlobalMetrics.TodoListByStatus.WithLabelValues(status).Set(0)
	}
}

// StartMetricsCollector 启动监控指标收集器
func StartMetricsCollector(ctx context.Context, todoService core.TodoListService) {
	ticker := time.NewTicker(30 * time.Second) // 每30秒更新一次指标
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("metrics collector stopped")
			return
		case <-ticker.C:
			UpdateTodoMetrics(ctx, todoService)
		}
	}
}
