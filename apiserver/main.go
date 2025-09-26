// Package main 计划任务系统 API Server 主程序
//
// 这是一个分布式计划任务系统的API服务器，提供以下核心功能：
// 1. 定时任务管理 (CronJob) - 支持cron表达式的时间调度
// 2. 任务执行记录 (Task) - 记录每次任务执行的详细信息
// 3. 工作节点管理 (Worker) - 管理执行任务的工作节点
// 4. 分布式锁机制 - 基于Redis的分布式锁，确保任务不重复执行
// 5. WebSocket实时通信 - 与Worker节点进行实时任务分发和状态同步
// 6. 任务调度服务 - 自动根据cron表达式创建和执行任务
//
// 系统架构：
// - API Server: 负责任务管理、调度和状态跟踪
// - Worker: 负责具体任务的执行
// - Redis: 提供分布式锁和缓存
// - MySQL/PostgreSQL: 持久化存储任务和配置数据

// @title           计划任务系统 API
// @version         1.0.0
// @description     分布式计划任务系统API服务器，提供定时任务管理、工作节点管理、任务调度等功能
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.example.com/support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath  /api/v1

// @securityDefinitions.apikey  BearerAuth
// @in                         header
// @name                       Authorization
// @description                JWT token, format: Bearer {token}
package main

import (
	"github.com/codelieche/cronjob/apiserver/pkg/app"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
)

// main 程序入口点
// 启动API服务器，包括：
// 1. 初始化日志系统
// 2. 启动Web服务器
// 3. 启动后台调度服务
// 4. 启动WebSocket服务
func main() {
	logger.Info("计划任务系统 API Server 启动中...")
	app.Run()
	logger.Info("计划任务系统 API Server 已停止")
}
