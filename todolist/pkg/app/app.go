// Package app 应用程序核心模块
//
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codelieche/todolist/pkg/config"
	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/middleware"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// newApp 创建并配置Gin Web应用实例
//
// 返回配置好的Gin引擎，包括：
// - 默认中间件（日志、恢复等）
// - CORS跨域中间件
// - 基础配置
//
// 返回值:
//   - *gin.Engine: 配置好的Gin引擎实例
func newApp() *gin.Engine {
	app := gin.Default()

	// 🔥 添加CORS中间件，解决跨域问题
	// 这个中间件必须在所有路由之前注册
	app.Use(middleware.CORSMiddleware())

	return app
}

// Run 启动API服务器
//
// 这是应用程序的主启动函数，执行以下步骤：
// 1. 初始化日志系统
// 2. 创建Gin Web应用实例
// 3. 初始化所有API路由
// 4. 启动Web服务器监听HTTP请求
// 5. 实现优雅关闭机制
//
// 注意：此函数会阻塞执行，直到收到关闭信号
func Run() {
	// 初始化日志系统
	logger.InitLogger()
	logger.Info("TodoList API Server 启动中", zap.String("监听地址", config.Web.Address()))

	// 创建Web应用实例
	app := newApp()

	// 初始化所有API路由
	// 包括：待办事项管理、健康检查等
	initRouter(app)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         config.Web.Address(),
		Handler:      app,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 在goroutine中启动服务器
	go func() {
		logger.Info("TodoList API Server 已启动", zap.String("监听地址", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("服务器启动失败", zap.Error(err))
		}
	}()

	// 实现优雅关闭
	gracefulShutdown(server)
}

// gracefulShutdown 优雅关闭服务器
func gracefulShutdown(server *http.Server) {
	// 创建信号通道
	quit := make(chan os.Signal, 1)
	// 监听系统信号
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-quit
	logger.Info("收到关闭信号，开始优雅关闭", zap.String("signal", sig.String()))

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	logger.Info("正在关闭HTTP服务器...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP服务器关闭失败", zap.Error(err))
	} else {
		logger.Info("HTTP服务器已关闭")
	}

	// 关闭数据库连接
	logger.Info("正在关闭数据库连接...")
	if err := core.CloseDB(); err != nil {
		logger.Error("数据库连接关闭失败", zap.Error(err))
	} else {
		logger.Info("数据库连接已关闭")
	}

	// 刷新日志缓冲区
	logger.Info("正在刷新日志缓冲区...")
	logger.Sync()

	logger.Info("TodoList API Server 已优雅关闭")
}
