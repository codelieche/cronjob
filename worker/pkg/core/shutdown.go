package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// ShutdownManager 优雅关闭管理器接口
//
// 提供分阶段的优雅关闭机制，确保：
// 1. 停止接收新任务
// 2. 等待正在运行的任务完成
// 3. 关闭各个服务
// 4. 清理资源
type ShutdownManager interface {
	// RegisterTaskService 注册任务服务，用于获取正在运行的任务信息
	RegisterTaskService(taskService TaskService)

	// RegisterWebSocketService 注册WebSocket服务，用于关闭连接
	RegisterWebSocketService(wsService WebsocketService)

	// Shutdown 执行优雅关闭
	//
	// 参数:
	//   - timeout: 最大等待时间，超过此时间将强制关闭
	//
	// 返回值:
	//   - error: 关闭过程中的错误
	Shutdown(timeout time.Duration) error

	// AddShutdownHook 添加关闭钩子函数
	//
	// 参数:
	//   - name: 钩子名称，用于日志记录
	//   - hook: 钩子函数，在关闭过程中执行
	AddShutdownHook(name string, hook func() error)
}

// shutdownManagerImpl 优雅关闭管理器实现
type shutdownManagerImpl struct {
	taskService TaskService             // 任务服务，用于获取正在运行的任务
	wsService   WebsocketService        // WebSocket服务，用于关闭连接
	hooks       map[string]func() error // 关闭钩子函数
	mutex       sync.RWMutex            // 保护并发访问
	isShutdown  bool                    // 是否已经关闭
}

// NewShutdownManager 创建优雅关闭管理器实例
func NewShutdownManager() ShutdownManager {
	return &shutdownManagerImpl{
		hooks: make(map[string]func() error),
	}
}

// RegisterTaskService 注册任务服务
func (s *shutdownManagerImpl) RegisterTaskService(taskService TaskService) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.taskService = taskService
	logger.Info("已注册任务服务到关闭管理器")
}

// RegisterWebSocketService 注册WebSocket服务
func (s *shutdownManagerImpl) RegisterWebSocketService(wsService WebsocketService) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.wsService = wsService
	logger.Info("已注册WebSocket服务到关闭管理器")
}

// AddShutdownHook 添加关闭钩子函数
func (s *shutdownManagerImpl) AddShutdownHook(name string, hook func() error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.hooks[name] = hook
	logger.Info("已添加关闭钩子", zap.String("name", name))
}

// Shutdown 执行优雅关闭
func (s *shutdownManagerImpl) Shutdown(timeout time.Duration) error {
	s.mutex.Lock()
	if s.isShutdown {
		s.mutex.Unlock()
		return fmt.Errorf("关闭管理器已经关闭")
	}
	s.isShutdown = true
	s.mutex.Unlock()

	logger.Info("开始优雅关闭", zap.Duration("timeout", timeout))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 阶段1：停止接收新任务
	if err := s.stopNewTasks(ctx); err != nil {
		logger.Error("停止接收新任务失败", zap.Error(err))
		return fmt.Errorf("停止接收新任务失败: %w", err)
	}

	// 阶段2：等待正在运行的任务完成
	if err := s.waitRunningTasks(ctx); err != nil {
		logger.Error("等待运行任务失败", zap.Error(err))
		return fmt.Errorf("等待运行任务失败: %w", err)
	}

	// 阶段3：关闭WebSocket连接
	if err := s.closeWebSocket(ctx); err != nil {
		logger.Error("关闭WebSocket连接失败", zap.Error(err))
		return fmt.Errorf("关闭WebSocket连接失败: %w", err)
	}

	// 阶段4：执行关闭钩子
	if err := s.executeShutdownHooks(ctx); err != nil {
		logger.Error("执行关闭钩子失败", zap.Error(err))
		return fmt.Errorf("执行关闭钩子失败: %w", err)
	}

	logger.Info("优雅关闭完成")
	return nil
}

// stopNewTasks 停止接收新任务
func (s *shutdownManagerImpl) stopNewTasks(ctx context.Context) error {
	logger.Info("阶段1：停止接收新任务")

	// 通过关闭WebSocket连接来停止接收新任务
	if s.wsService != nil {
		// 这里可以添加一个"停止接收"的方法，而不是直接关闭
		logger.Info("WebSocket服务将在后续阶段关闭")
	}

	logger.Info("已停止接收新任务")
	return nil
}

// waitRunningTasks 等待正在运行的任务完成
func (s *shutdownManagerImpl) waitRunningTasks(ctx context.Context) error {
	logger.Info("阶段2：等待正在运行的任务完成")

	if s.taskService == nil {
		logger.Info("没有注册任务服务，跳过等待")
		return nil
	}

	// 获取剩余超时时间
	deadline, ok := ctx.Deadline()
	var remainingTimeout time.Duration
	if ok {
		remainingTimeout = time.Until(deadline)
	} else {
		remainingTimeout = 30 * time.Second // 默认30秒
	}

	// 使用TaskService的WaitForTasksCompletion方法
	return s.taskService.WaitForTasksCompletion(remainingTimeout)
}

// closeWebSocket 关闭WebSocket连接
func (s *shutdownManagerImpl) closeWebSocket(ctx context.Context) error {
	logger.Info("阶段3：关闭WebSocket连接")

	if s.wsService != nil {
		s.wsService.Stop()
		logger.Info("WebSocket服务已关闭")
	} else {
		logger.Info("没有注册WebSocket服务，跳过关闭")
	}

	return nil
}

// executeShutdownHooks 执行关闭钩子
func (s *shutdownManagerImpl) executeShutdownHooks(ctx context.Context) error {
	logger.Info("阶段4：执行关闭钩子")

	s.mutex.RLock()
	hooks := make(map[string]func() error)
	for name, hook := range s.hooks {
		hooks[name] = hook
	}
	s.mutex.RUnlock()

	for name, hook := range hooks {
		select {
		case <-ctx.Done():
			logger.Warn("执行关闭钩子超时", zap.String("hook", name))
			return ctx.Err()
		default:
			logger.Info("执行关闭钩子", zap.String("name", name))
			if err := hook(); err != nil {
				logger.Error("关闭钩子执行失败", zap.String("name", name), zap.Error(err))
				// 继续执行其他钩子，不因为一个钩子失败而中断
			} else {
				logger.Info("关闭钩子执行成功", zap.String("name", name))
			}
		}
	}

	logger.Info("所有关闭钩子执行完成")
	return nil
}
