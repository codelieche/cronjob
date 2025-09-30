// Package app 权限注册
//
// 负责在系统启动时向usercenter注册权限、角色和平台配置
package app

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// getSystemPermissions 获取系统权限定义
func getSystemPermissions() *core.PermissionRegistryRequest {
	return &core.PermissionRegistryRequest{
		SystemCode: config.SystemCode,
		Permissions: []core.PermissionItem{
			// 任务分类权限
			{
				Code:        "category.view",
				Name:        "查看分类",
				Description: "查看任务分类列表和详情的权限",
			},
			{
				Code:        "category.create",
				Name:        "创建分类",
				Description: "创建新任务分类的权限",
			},
			{
				Code:        "category.edit",
				Name:        "编辑分类",
				Description: "编辑现有任务分类的权限",
			},
			{
				Code:        "category.delete",
				Name:        "删除分类",
				Description: "删除任务分类的权限",
			},

			// 定时任务权限
			{
				Code:        "cronjob.view",
				Name:        "查看任务",
				Description: "查看定时任务列表和详情的权限",
			},
			{
				Code:        "cronjob.create",
				Name:        "创建任务",
				Description: "创建新定时任务的权限",
			},
			{
				Code:        "cronjob.edit",
				Name:        "编辑任务",
				Description: "编辑现有定时任务的权限",
			},
			{
				Code:        "cronjob.delete",
				Name:        "删除任务",
				Description: "删除定时任务的权限",
			},
			{
				Code:        "cronjob.execute",
				Name:        "执行任务",
				Description: "手动执行定时任务的权限",
			},
			{
				Code:        "cronjob.enable",
				Name:        "启用任务",
				Description: "启用/禁用定时任务的权限",
			},

			// 任务执行记录权限
			{
				Code:        "task.view",
				Name:        "查看执行记录",
				Description: "查看任务执行记录的权限",
			},
			{
				Code:        "task.delete",
				Name:        "删除执行记录",
				Description: "删除任务执行记录的权限",
			},
			{
				Code:        "task.kill",
				Name:        "终止任务",
				Description: "终止正在执行的任务的权限",
			},

			// 任务日志权限
			{
				Code:        "tasklog.view",
				Name:        "查看任务日志",
				Description: "查看任务执行日志的权限",
			},
			{
				Code:        "tasklog.delete",
				Name:        "删除任务日志",
				Description: "删除任务执行日志的权限",
			},

			// Worker节点权限
			{
				Code:        "worker.view",
				Name:        "查看Worker",
				Description: "查看Worker节点信息的权限",
			},
			{
				Code:        "worker.manage",
				Name:        "管理Worker",
				Description: "管理Worker节点的权限",
			},

			// 系统监控权限
			{
				Code:        "metrics.view",
				Name:        "查看监控",
				Description: "查看系统监控指标的权限",
			},

			// WebSocket权限
			{
				Code:        "websocket.connect",
				Name:        "WebSocket连接",
				Description: "建立WebSocket连接的权限",
			},
		},
		Roles: []core.RoleItem{
			{
				Code:        "admin",
				Name:        "系统管理员",
				Description: "拥有所有系统管理权限",
				Permissions: []string{
					"category.view", "category.create", "category.edit", "category.delete",
					"cronjob.view", "cronjob.create", "cronjob.edit", "cronjob.delete", "cronjob.execute", "cronjob.enable",
					"task.view", "task.delete", "task.kill",
					"tasklog.view", "tasklog.delete",
					"worker.view", "worker.manage",
					"metrics.view",
					"websocket.connect",
				},
			},
			{
				Code:        "operator",
				Name:        "运维操作员",
				Description: "拥有任务操作和监控权限",
				Permissions: []string{
					"category.view",
					"cronjob.view", "cronjob.execute", "cronjob.enable",
					"task.view", "task.kill",
					"tasklog.view",
					"worker.view",
					"metrics.view",
					"websocket.connect",
				},
			},
			{
				Code:        "developer",
				Name:        "开发者",
				Description: "拥有任务开发和管理权限",
				Permissions: []string{
					"category.view", "category.create", "category.edit",
					"cronjob.view", "cronjob.create", "cronjob.edit", "cronjob.execute",
					"task.view",
					"tasklog.view",
					"websocket.connect",
				},
			},
			{
				Code:        "viewer",
				Name:        "只读用户",
				Description: "只能查看系统信息",
				Permissions: []string{
					"category.view",
					"cronjob.view",
					"task.view",
					"tasklog.view",
					"worker.view",
					"metrics.view",
				},
			},
		},
	}
}

// getSystemPlatforms 获取系统平台定义
func getSystemPlatforms() *core.PlatformRegistryRequest {
	return &core.PlatformRegistryRequest{
		SystemCode: config.SystemCode,
		Platforms: []core.PlatformItem{
			{
				Name:           "cronjob-admin",
				Title:          "定时任务管理",
				Icon:           "icon-schedule",
				Path:           "/cronjob",
				Description:    "定时任务管理平台",
				Order:          20,
				PermissionCode: "cronjob.view",
				IsMenu:         true,
				IsFrontend:     true,
				Server:         "http://localhost:3002",
				Container:      "#cronjob-container",
				ActiveRule:     "/cronjob",
			},
			{
				Name:           "cronjob-monitor",
				Title:          "任务监控",
				Icon:           "icon-monitor",
				Path:           "/cronjob/monitor",
				Description:    "定时任务监控平台",
				Order:          21,
				PermissionCode: "metrics.view",
				IsMenu:         true,
				IsFrontend:     false,
				Server:         "",
				Container:      "",
				ActiveRule:     "/cronjob/monitor",
			},
			{
				Name:           "cronjob-logs",
				Title:          "任务日志",
				Icon:           "icon-logs",
				Path:           "/cronjob/logs",
				Description:    "定时任务日志查看",
				Order:          22,
				PermissionCode: "tasklog.view",
				IsMenu:         true,
				IsFrontend:     false,
				Server:         "",
				Container:      "",
				ActiveRule:     "/cronjob/logs",
			},
		},
	}
}

// RegisterPermissions 注册权限和角色到用户中心
func RegisterPermissions() error {
	logger.Info("开始注册权限和角色到用户中心")

	// 创建注册服务
	registryService := services.NewRegistryService()

	// 创建上下文，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取系统权限定义
	permissionsReq := getSystemPermissions()

	// 注册权限和角色
	resp, err := registryService.RegisterPermissions(ctx, permissionsReq)
	if err != nil {
		logger.Error("注册权限和角色失败", zap.Error(err))
		return err
	}

	logger.Info("权限和角色注册成功",
		zap.String("message", resp.Message),
		zap.String("system_code", resp.SystemCode),
		zap.Int("permissions", resp.Permissions),
		zap.Int("roles", resp.Roles),
	)

	return nil
}

// RegisterPlatforms 注册平台配置到用户中心
func RegisterPlatforms() error {
	logger.Info("开始注册平台配置到用户中心")

	// 创建注册服务
	registryService := services.NewRegistryService()

	// 创建上下文，设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取系统平台定义
	platformsReq := getSystemPlatforms()

	// 注册平台
	resp, err := registryService.RegisterPlatforms(ctx, platformsReq)
	if err != nil {
		logger.Error("注册平台配置失败", zap.Error(err))
		return err
	}

	logger.Info("平台配置注册成功",
		zap.String("message", resp.Message),
		zap.String("system_code", resp.SystemCode),
		zap.Int("platforms", resp.Platforms),
	)

	return nil
}

// RegisterAll 注册所有权限和平台配置
func RegisterAll() error {
	logger.Info("开始注册所有权限和平台配置到用户中心")

	// 注册权限和角色
	if err := RegisterPermissions(); err != nil {
		logger.Error("注册权限和角色失败", zap.Error(err))
		// 不直接返回错误，继续尝试注册平台
	}

	// 注册平台配置
	if err := RegisterPlatforms(); err != nil {
		logger.Error("注册平台配置失败", zap.Error(err))
		// 不直接返回错误，记录日志即可
	}

	logger.Info("权限和平台配置注册完成")
	return nil
}

// RegisterWithRetry 带重试机制的注册
func RegisterWithRetry(maxRetries int, retryDelay time.Duration) error {
	if config.Auth.ApiKey == "" {
		logger.Error("认证服务API密钥为空，跳过注册")
		return nil
	}

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logger.Warn("重试注册权限和平台配置",
				zap.Int("attempt", i+1),
				zap.Int("max_retries", maxRetries),
				zap.Error(lastErr),
			)
			time.Sleep(retryDelay)
		}

		err := RegisterAll()
		if err == nil {
			logger.Info("权限和平台配置注册成功", zap.Int("attempts", i+1))
			return nil
		}

		lastErr = err
	}

	logger.Error("权限和平台配置注册最终失败",
		zap.Int("max_retries", maxRetries),
		zap.Error(lastErr),
	)

	return lastErr
}
