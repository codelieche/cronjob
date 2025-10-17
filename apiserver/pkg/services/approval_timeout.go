package services

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// StartApprovalTimeoutChecker 启动审批超时检查器（后台定时任务）
// 定期检查超时的审批，自动设置为timeout状态
func StartApprovalTimeoutChecker(ctx context.Context, approvalService *ApprovalService) error {
	// 配置检查间隔：每5分钟检查一次
	checkInterval := 5 * time.Minute

	logger.Info("启动审批超时检查器",
		zap.Duration("check_interval", checkInterval))

	// 创建定时器
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// 立即执行一次
	if err := approvalService.HandleTimeout(ctx); err != nil {
		logger.Error("审批超时检查失败", zap.Error(err))
	}

	// 循环检查
	for {
		select {
		case <-ctx.Done():
			logger.Info("审批超时检查器已停止")
			return ctx.Err()
		case <-ticker.C:
			// 定时触发检查
			logger.Debug("开始检查审批超时")
			if err := approvalService.HandleTimeout(ctx); err != nil {
				logger.Error("审批超时检查失败", zap.Error(err))
			}
		}
	}
}
