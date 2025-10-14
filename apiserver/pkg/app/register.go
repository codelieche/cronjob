// Package app Category注册
//
// 负责在系统启动时注册系统分类（Category）
//
// 注意：权限和平台配置已统一由 usercenter 管理
// 如需修改权限定义，请编辑：usercenter/pkg/services/register.go
package app

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RegisterCategories 注册系统分类
func RegisterCategories(db *gorm.DB) error {
	logger.Info("开始注册系统分类")

	// 创建 CategoryStore 和 CategoryService
	categoryStore := store.NewCategoryStore(db)
	categoryService := services.NewCategoryService(categoryStore)

	// 创建带CategoryService的注册服务
	registryService := services.NewRegistryServiceWithCategory(categoryService)

	// 创建上下文，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 注册分类
	err := registryService.RegisterCategories(ctx)
	if err != nil {
		logger.Error("注册系统分类失败", zap.Error(err))
		return err
	}

	logger.Info("系统分类注册成功")
	return nil
}

// RegisterCategoriesWithRetry 带重试机制的分类注册
func RegisterCategoriesWithRetry(db *gorm.DB, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logger.Warn("重试注册系统分类",
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries),
				zap.Error(lastErr),
			)
			time.Sleep(retryDelay)
		}

		err := RegisterCategories(db)
		if err == nil {
			logger.Info("系统分类注册成功", zap.Int("attempts", i+1))
			return nil
		}

		lastErr = err
	}

	logger.Error("系统分类注册最终失败",
		zap.Int("max_retries", maxRetries),
		zap.Error(lastErr),
	)

	return lastErr
}
