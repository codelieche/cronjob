// Package app 应用程序核心模块
// 
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// dispatch 启动后台调度服务
// 
// 此函数负责启动所有后台服务，包括：
// 1. 定时任务调度循环 - 根据cron表达式创建任务
// 2. 任务超时检查循环 - 检查并处理超时的任务
// 3. WebSocket队列消费 - 处理任务分发和状态同步
//
// 这些服务在独立的goroutine中运行，不会阻塞主线程
func dispatch() {
	// 获取数据库连接
	db, err := core.GetDB()
	if err != nil {
		logger.Panic("获取数据库连接失败", zap.Error(err))
	}
	
	// 初始化数据存储层
	cronJobStore := store.NewCronJobStore(db)  // 定时任务存储
	taskStore := store.NewTaskStore(db)        // 任务记录存储
	workerStore := store.NewWorkerStore(db)    // 工作节点存储
	
	// 初始化Redis分布式锁服务
	lockerService, err := services.NewRedisLocker()
	if err != nil {
		logger.Panic("创建Redis分布式锁服务失败", zap.Error(err))
	}

	// 创建任务调度服务
	// 负责根据cron表达式创建任务，并管理任务的生命周期
	dispatchService := services.NewDispatchService(
		cronJobStore, taskStore, lockerService,
	)

	// 启动定时任务调度循环
	// 在独立goroutine中运行，持续检查需要调度的定时任务
	go dispatchService.DispatchLoop(context.Background())
	logger.Info("定时任务调度循环已启动")

	// 启动任务超时检查循环
	// 在独立goroutine中运行，持续检查超时的任务
	go dispatchService.CheckTaskLoop(context.Background())
	logger.Info("任务超时检查循环已启动")

	// 创建WebSocket服务
	// 负责与Worker节点进行实时通信
	websocketService := services.NewWebsocketService(taskStore, workerStore)
	
	// 启动WebSocket队列消费服务
	// 在独立goroutine中运行，处理任务分发和状态同步
	go websocketService.StartConsumingQueues()
	logger.Info("WebSocket队列消费服务已启动")
	
	logger.Info("所有后台调度服务启动完成")
}
