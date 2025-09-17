// Package app 应用程序核心模块
// 
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// newApp 创建并配置Gin Web应用实例
// 
// 返回配置好的Gin引擎，包括：
// - 默认中间件（日志、恢复等）
// - 基础配置
//
// 返回值:
//   - *gin.Engine: 配置好的Gin引擎实例
func newApp() *gin.Engine {
	app := gin.Default()
	return app
}

// Run 启动API服务器
// 
// 这是应用程序的主启动函数，执行以下步骤：
// 1. 初始化日志系统
// 2. 创建Gin Web应用实例
// 3. 初始化所有API路由
// 4. 启动后台调度服务（定时任务调度、任务检查等）
// 5. 启动Web服务器监听HTTP请求
//
// 注意：此函数会阻塞执行，直到服务器停止
func Run() {
	// 初始化日志系统
	logger.InitLogger()
	logger.Info("计划任务系统 API Server 启动中", zap.String("监听地址", config.Web.Address()))
	
	// 创建Web应用实例
	app := newApp()

	// 初始化所有API路由
	// 包括：用户管理、工作节点、分类、定时任务、任务记录、分布式锁、WebSocket等
	initRouter(app)

	// 启动后台服务
	// 包括：定时任务调度循环、任务超时检查循环、WebSocket队列消费等
	dispatch()

	// 启动Web服务器，开始监听HTTP请求
	// 此调用会阻塞，直到服务器停止
	app.Run(config.Web.Address())
}
