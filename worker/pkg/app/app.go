package app

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/services"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// App Worker应用结构体
type App struct {
	wsService core.WebsocketService
	done      chan struct{}
}

// NewApp 创建新的Worker应用实例
func NewApp() *App {
	return &App{
		done: make(chan struct{}),
	}
}

// Initialize 初始化应用
func (a *App) Initialize() error {
	// 初始化日志
	logger.InitLogger()
	logger.Info("Worker启动中...")

	// 打印配置信息
	a.printConfig()

	// 创建WebSocket服务
	a.wsService = services.NewWebsocketService()

	return nil
}

// printConfig 打印配置信息
func (a *App) printConfig() {
	logger.Info("Worker配置信息",
		zap.String("name", config.WorkerInstance.Name),
		zap.String("id", config.WorkerInstance.ID.String()),
		zap.String("server_url", config.Server.ApiUrl),
		zap.Int("ping_interval", config.WebsocketPingInterval))
}

// Start 启动应用
func (a *App) Start() error {
	// 启动WebSocket服务
	if err := a.wsService.Start(); err != nil {
		logger.Fatal("启动WebSocket服务失败", zap.Error(err))
		return err
	}

	logger.Info("Worker启动成功")
	return nil
}

// Run 运行应用主循环
func (a *App) Run() {
	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case <-quit:
		logger.Info("收到退出信号")
	case <-a.done:
		logger.Info("收到内部退出信号")
	}

	// 优雅关闭
	a.Shutdown()
}

// Shutdown 优雅关闭应用
func (a *App) Shutdown() {
	logger.Info("Worker正在关闭...")
	
	if a.wsService != nil {
		a.wsService.Stop()
	}
	
	logger.Info("Worker已关闭")
}

// Stop 停止应用
func (a *App) Stop() {
	close(a.done)
}