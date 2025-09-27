package services

import (
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
)

// CreateServices 创建所有服务实例，解决循环依赖问题
//
// 使用工厂模式统一创建和配置服务，确保依赖关系正确
//
// 返回值:
//   - core.WebsocketService: WebSocket服务实例
//   - core.TaskService: 任务服务实例
func CreateServices() (core.WebsocketService, core.TaskService) {
	// 创建API服务器通信服务
	apiserver := NewApiserverService(config.Server.ApiUrl, config.Server.ApiKey)

	// 创建WebSocket服务实现（不依赖TaskService）
	wsService := &WebsocketServiceImpl{
		config:    createWebSocketConfig(),
		done:      make(chan struct{}),
		apiserver: apiserver,
	}

	// 创建任务服务实现（依赖WebSocket服务作为回调）
	taskService := &TaskServiceImpl{
		updateCallback: wsService,                       // WebSocket服务实现了TaskUpdateCallback
		errorHandler:   core.NewErrorHandler(wsService), // 错误处理器使用WebSocket回调
		locker:         NewLocker(),                     // 分布式锁服务
		apiserver:      apiserver,                       // API服务器通信
		runningTasks:   make(map[string]core.Runner),    // 运行中的任务映射
	}

	// 设置WebSocket服务的任务事件处理器
	wsService.eventHandler = taskService

	return wsService, taskService
}

// createWebSocketConfig 创建WebSocket配置
func createWebSocketConfig() *core.WebsocketConfig {
	wsConfig := core.DefaultWebsocketConfig()
	wsConfig.ServerURL = config.Server.ApiUrl
	wsConfig.PingInterval = time.Duration(config.WebsocketPingInterval) * time.Second
	wsConfig.MessageSeparator = config.WebsocketMessageSeparator
	return wsConfig
}
