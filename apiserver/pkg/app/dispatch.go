// Package app 应用程序核心模块
//
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/monitoring"
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
	cronJobStore := store.NewCronJobStore(db) // 定时任务存储
	taskStore := store.NewTaskStore(db)       // 任务记录存储
	workerStore := store.NewWorkerStore(db)   // 工作节点存储

	// 初始化Redis分布式锁服务
	lockerService, err := services.NewRedisLocker()
	if err != nil {
		logger.Panic("创建Redis分布式锁服务失败", zap.Error(err))
	}

	// 🔥 创建CronJob服务（用于Scheduler调用）
	cronJobService := services.NewCronJobService(cronJobStore)

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

	// 🔥 启动失败任务自动重试循环
	// 在独立goroutine中运行，持续检查失败的可重试任务
	go dispatchService.CheckFailedTasksLoop(context.Background())
	logger.Info("失败任务自动重试循环已启动")

	// 创建WebSocket服务
	// 负责与Worker节点进行实时通信
	websocketService := services.NewWebsocketService(taskStore, workerStore)

	// 启动WebSocket队列消费服务
	// 在独立goroutine中运行，处理任务分发和状态同步
	go websocketService.StartConsumingQueues()
	logger.Info("WebSocket队列消费服务已启动")

	// 启动业务指标收集器
	// 定期收集CronJob、Task等业务指标
	businessCollector := monitoring.NewBusinessMetricsCollector(cronJobStore, taskStore, 30*time.Second)
	go businessCollector.Start(context.Background())
	logger.Info("业务指标收集器已启动")

	// 启动数据库指标收集器
	// 定期收集数据库连接池等指标
	dbCollector := monitoring.NewDatabaseMetricsCollector(60 * time.Second)
	go dbCollector.Start(context.Background())
	logger.Info("数据库指标收集器已启动")

	// 启动Worker状态检查循环
	// 定期检查worker的last_active，将超过5分钟没有心跳的worker标记为失活
	workerService := services.NewWorkerService(workerStore)
	go workerService.CheckWorkerStatusLoop(
		context.Background(),
		30*time.Second, // 每30秒检查一次
		5*time.Minute,  // 超过5分钟没有心跳的worker视为失活
	)
	logger.Info("Worker状态检查循环已启动")

	// 🔥 启动后台定时任务调度器（Cron-based）
	// 与上面的"定时任务调度循环"不同，这是基于Cron表达式的系统维护任务
	// 包含任务：
	// 1. 统计数据聚合（凌晨01:00，P2架构优化）
	// 2. CronJob初始化（每10分钟，P5优化）
	// 3. TaskLog分片维护（凌晨02:00）
	// 使用分布式锁防止多副本并发执行
	// 架构层次：Scheduler -> Service -> Store -> Database
	scheduler := NewScheduler(db, lockerService, cronJobService)
	go func() {
		if err := scheduler.Start(); err != nil {
			logger.Error("启动定时任务调度器失败", zap.Error(err))
		} else {
			logger.Info("定时任务调度器已启动（统一管理所有定时任务，分布式锁保护）")
		}
	}()

	logger.Info("所有后台调度服务启动完成")
}
