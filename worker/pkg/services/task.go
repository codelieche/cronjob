// Package services Worker业务服务层
//
// 实现Worker节点的核心业务逻辑，包括：
// - API Server通信服务：与API Server进行HTTP通信
// - WebSocket服务：与API Server进行实时通信
// - 任务执行服务：执行具体的任务
// - 分布式锁服务：确保任务不重复执行
package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	_ "github.com/codelieche/cronjob/worker/pkg/runner" // 导入runner包以触发init函数注册
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// taskServiceImpl 任务执行服务实现
//
// 负责处理从API Server接收到的任务，包括：
// - 任务执行逻辑
// - 状态更新和结果上报
// - 分布式锁管理
// - 任务重试和超时处理
type taskServiceImpl struct {
	wsService    core.WebsocketService  // WebSocket服务，用于与API Server通信
	locker       core.Locker            // 分布式锁服务，确保任务不重复执行
	apiserver    core.Apiserver         // API Server通信服务，用于获取任务详情
	runningTasks map[string]core.Runner // 正在运行的任务，key为task_id
	taskMutex    sync.RWMutex           // 保护runningTasks的并发访问
}

// NewTaskService 创建任务执行服务实例
//
// 参数:
//   - wsService: WebSocket服务，用于与API Server通信
//
// 返回值:
//   - core.TaskService: 任务执行服务接口
func NewTaskService(wsService core.WebsocketService, apiserver core.Apiserver) core.TaskService {
	return &taskServiceImpl{
		wsService:    wsService,
		locker:       NewLocker(),                  // 创建分布式锁服务实例
		apiserver:    apiserver,                    // API Server通信服务，用于获取任务详情
		runningTasks: make(map[string]core.Runner), // 初始化正在运行的任务映射
	}
}

// HandleTaskEvent 处理任务事件
//
// 根据事件类型分发到相应的处理方法
// 支持的事件类型：运行、停止、终止、超时、重试
//
// 参数:
//   - event: 任务事件对象，包含事件类型和任务列表
func (ts *taskServiceImpl) HandleTaskEvent(event *core.TaskEvent) {
	logger.Info("收到任务事件", zap.String("action", event.Action), zap.Int("task_count", len(event.Tasks)))

	switch event.Action {
	case string(core.TaskActionRun):
		ts.RunTasks(event.Tasks) // 运行任务
	case string(core.TaskActionStop):
		ts.StopTasks(event.Tasks) // 停止任务
	case string(core.TaskActionKill):
		ts.KillTasks(event.Tasks) // 强制终止任务
	case string(core.TaskActionTimeout):
		ts.TimeoutTasks(event.Tasks) // 处理超时任务
	case string(core.TaskActionRetry):
		ts.RetryTasks(event.Tasks) // 重试任务
	default:
		logger.Warn("未知的任务事件类型", zap.String("action", event.Action))
	}
}

// ExecuteTask 执行任务
//
// 检查任务类型是否支持，如果支持则异步执行任务
// 使用分布式锁确保同一任务不会被多个Worker同时执行
//
// 参数:
//   - task: 要执行的任务对象
func (ts *taskServiceImpl) ExecuteTask(task *core.Task) {
	logger.Info("开始处理任务",
		zap.String("task_id", task.ID.String()),
		zap.String("task_name", task.Name),
		zap.String("category", task.Category),
		zap.String("command", task.Command))

	// 检查任务类型是否在Worker支持的任务列表中
	if !ts.isTaskCategorySupported(task.Category) {
		// 直接跳过不支持的任务类型
		logger.Debug("跳过不支持的任务类型", zap.String("category", task.Category))
		return
	}

	// 异步执行任务，避免阻塞主线程
	// 在executeTask中会获取分布式锁
	go ts.executeTask(task)
}

// executeTask 执行任务的具体实现
func (ts *taskServiceImpl) executeTask(task *core.Task) {
	// 生成锁的key
	lockKey := fmt.Sprintf(config.TaskLockerKeyFormat, task.ID.String())

	// 尝试获取锁
	ctx := context.Background()
	lock, err := ts.locker.TryAcquire(ctx, lockKey, 6*time.Second)
	if err != nil {
		// 没获取到锁，就不用管跳过
		return
	}

	logger.Info("成功获取任务锁，开始执行任务",
		zap.String("task_id", task.ID.String()),
		zap.String("task_name", task.Name),
		zap.String("category", task.Category),
		zap.String("lock_key", lockKey),
		zap.String("command", task.Command))

	// 启动自动续租
	refreshInterval := 3 * time.Second // 续租间隔应该小于锁的过期时间
	stopRefresh, err := lock.AutoRefresh(ctx, 6*time.Second, refreshInterval)
	if err != nil {
		logger.Error("启动自动续租失败",
			zap.String("task_id", task.ID.String()),
			zap.Error(err))
		// 释放锁
		lock.Release(ctx)
		return
	}

	// 确保在函数结束时停止续租和释放锁
	defer func() {
		stopRefresh()
		if err := lock.Release(ctx); err != nil {
			logger.Error("释放任务锁失败",
				zap.String("task_id", task.ID.String()),
				zap.Error(err))
		} else {
			logger.Info("成功释放任务锁",
				zap.String("task_id", task.ID.String()))
		}
	}()

	// 获取任务详情
	taskDetail, err := ts.apiserver.GetTask(task.ID.String())
	if err != nil {
		logger.Error("获取任务详情失败",
			zap.String("task_id", task.ID.String()),
			zap.Error(err))
		return
	} else {
		if taskDetail != nil && taskDetail.Status != core.TaskStatusPending {
			logger.Info("任务状态不是pending，跳过执行: "+taskDetail.Status,
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))
			return
		}
	}

	// 更新任务状态为运行中
	taskStart := map[string]interface{}{
		"status":      "running",
		"time_start":  time.Now().Format("2006-01-02"),
		"worker_id":   config.WorkerInstance.ID.String(),
		"worker_name": config.WorkerInstance.Name,
	}
	logger.Info(config.WorkerInstance.ID.String())
	ts.wsService.SendTaskUpdate(task.ID.String(), taskStart)

	// 创建并执行任务
	var taskResult *core.Result

	// 根据任务类型创建相应的Runner
	runnerInstance, err := core.CreateRunner(task.Category)
	if err != nil {
		logger.Error("创建Runner失败",
			zap.String("task_id", task.ID.String()),
			zap.String("category", task.Category),
			zap.Error(err))

		// 发送错误结果
		errorResult := map[string]interface{}{
			"status":   core.TaskStatusError,
			"output":   fmt.Sprintf("不支持的任务类型: %s", task.Category),
			"time_end": time.Now().Format("2006-01-02 15:04:05"),
		}
		ts.wsService.SendTaskUpdate(task.ID.String(), errorResult)
		return
	}

	// 解析任务参数
	if err := runnerInstance.ParseArgs(task.Command, task.Args); err != nil {
		logger.Error("解析任务参数失败",
			zap.String("task_id", task.ID.String()),
			zap.String("command", task.Command),
			zap.String("args", task.Args),
			zap.Error(err))

		// 发送错误结果
		errorResult := map[string]interface{}{
			"status":   core.TaskStatusError,
			"output":   fmt.Sprintf("参数解析失败: %v", err),
			"time_end": time.Now().Format("2006-01-02 15:04:05"),
		}
		ts.wsService.SendTaskUpdate(task.ID.String(), errorResult)
		runnerInstance.Cleanup()
		return
	}

	// 设置超时时间
	if task.Timeout > 0 {
		runnerInstance.SetTimeout(time.Duration(task.Timeout) * time.Second)
	}

	// 将Runner添加到正在运行的任务列表
	taskID := task.ID.String()
	ts.taskMutex.Lock()
	ts.runningTasks[taskID] = runnerInstance
	ts.taskMutex.Unlock()

	// 确保从正在运行的任务列表中移除并清理Runner资源
	defer func() {
		ts.taskMutex.Lock()
		delete(ts.runningTasks, taskID)
		ts.taskMutex.Unlock()

		if cleanupErr := runnerInstance.Cleanup(); cleanupErr != nil {
			logger.Error("清理Runner资源失败",
				zap.String("task_id", task.ID.String()),
				zap.Error(cleanupErr))
		}
	}()

	// 执行任务
	logger.Info("开始执行任务",
		zap.String("task_id", task.ID.String()),
		zap.String("category", task.Category),
		zap.String("command", task.Command),
		zap.Int("timeout", task.Timeout))

	taskResult, err = runnerInstance.Execute(ctx)

	// 处理执行结果
	if err != nil {
		logger.Error("任务执行失败",
			zap.String("task_id", task.ID.String()),
			zap.Error(err))

		// 发送错误结果
		errorResult := map[string]interface{}{
			"status":   core.TaskStatusError,
			"error":    err.Error(),
			"time_end": time.Now().Format("2006-01-02 15:04:05"),
		}
		ts.wsService.SendTaskUpdate(task.ID.String(), errorResult)
		return
	}

	// 处理Runner返回的结果
	if taskResult == nil {
		logger.Error("任务执行结果为空",
			zap.String("task_id", task.ID.String()))

		errorResult := map[string]interface{}{
			"status":   core.TaskStatusError,
			"error":    "任务执行结果为空",
			"time_end": time.Now().Format("2006-01-02 15:04:05"),
		}
		ts.wsService.SendTaskUpdate(task.ID.String(), errorResult)
		return
	}

	// 根据Runner结果状态映射到Task状态
	var taskStatus string
	switch taskResult.Status {
	case core.StatusSuccess:
		taskStatus = core.TaskStatusSuccess
	case core.StatusFailed:
		taskStatus = core.TaskStatusFailed
	case core.StatusTimeout:
		taskStatus = core.TaskStatusTimeout
	case core.StatusCanceled:
		taskStatus = core.TaskStatusCanceled
	case core.StatusError:
		taskStatus = core.TaskStatusError
	default:
		taskStatus = core.TaskStatusError
	}

	// 构建结果数据
	result := map[string]interface{}{
		"status":   taskStatus,
		"time_end": taskResult.EndTime.Format("2006-01-02 15:04:05"),
	}

	// 添加输出信息
	if taskResult.Output != "" {
		result["output"] = taskResult.Output
	}
	if taskStatus != core.TaskStatusSuccess {
		// 任务失败，添加错误信息
		if taskResult.Error != "" {
			if output, ok := result["output"].(string); ok {
				result["output"] = output + "\n\n-------\n\n" + taskResult.Error
			} else {
				result["output"] = taskResult.Error
			}
		}
	}

	// 添加错误信息
	if taskResult.Error != "" {
		result["error"] = taskResult.Error
	}

	// 添加执行时长
	if taskResult.Duration > 0 {
		result["duration"] = taskResult.Duration
	}

	// 添加退出码
	if taskResult.ExitCode != 0 {
		result["exit_code"] = taskResult.ExitCode
	}

	// 发送任务执行结果
	ts.wsService.SendTaskUpdate(task.ID.String(), result)

	// 记录执行结果日志
	if taskStatus == core.TaskStatusSuccess {
		logger.Info("任务执行成功",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
			zap.String("status", taskStatus),
			zap.Int64("duration", taskResult.Duration),
			zap.Int("exit_code", taskResult.ExitCode))
	} else {
		logger.Error("任务执行失败",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
			zap.String("status", taskStatus),
			zap.String("error", taskResult.Error),
			zap.Int("exit_code", taskResult.ExitCode))
	}
}

// isTaskCategorySupported 检查任务类型是否被Worker支持
func (ts *taskServiceImpl) isTaskCategorySupported(category string) bool {
	supportedTasks := config.WorkerInstance.Metadata.Tasks
	for _, supportedCategory := range supportedTasks {
		if supportedCategory == category {
			return true
		}
	}
	return false
}

// RunTasks 运行任务列表
func (ts *taskServiceImpl) RunTasks(tasks []*core.Task) {
	for _, task := range tasks {
		ts.ExecuteTask(task)
	}
}

// StopTasks 停止任务列表
func (ts *taskServiceImpl) StopTasks(tasks []*core.Task) {
	for _, task := range tasks {
		taskID := task.ID.String()

		logger.Info("停止任务",
			zap.String("task_id", taskID),
			zap.String("task_name", task.Name))

		// 查找正在运行的任务
		ts.taskMutex.RLock()
		runnerInstance, exists := ts.runningTasks[taskID]
		ts.taskMutex.RUnlock()

		if !exists {
			logger.Warn("任务不在运行列表中，可能已经完成",
				zap.String("task_id", taskID))

			// 发送任务已停止结果
			result := map[string]interface{}{
				"status":   core.TaskStatusCanceled,
				"time_end": time.Now().Format("2006-01-02 15:04:05"),
			}
			ts.wsService.SendTaskUpdate(taskID, result)
			continue
		}

		// 尝试优雅停止任务
		if err := runnerInstance.Stop(); err != nil {
			logger.Error("停止任务失败",
				zap.String("task_id", taskID),
				zap.Error(err))

			// 发送停止失败结果
			result := map[string]interface{}{
				"status":   core.TaskStatusError,
				"error":    fmt.Sprintf("停止任务失败: %v", err),
				"time_end": time.Now().Format("2006-01-02 15:04:05"),
			}
			ts.wsService.SendTaskUpdate(taskID, result)
		} else {
			logger.Info("任务停止请求已发送",
				zap.String("task_id", taskID))
		}
	}
}

// KillTasks 强制终止任务列表
func (ts *taskServiceImpl) KillTasks(tasks []*core.Task) {
	for _, task := range tasks {
		taskID := task.ID.String()

		logger.Info("强制终止任务",
			zap.String("task_id", taskID),
			zap.String("task_name", task.Name))

		// 查找正在运行的任务
		ts.taskMutex.RLock()
		runnerInstance, exists := ts.runningTasks[taskID]
		ts.taskMutex.RUnlock()

		if !exists {
			logger.Warn("任务不在运行列表中，可能已经完成",
				zap.String("task_id", taskID))

			// 发送任务已终止结果
			result := map[string]interface{}{
				"status":   core.TaskStatusCanceled,
				"time_end": time.Now().Format("2006-01-02 15:04:05"),
			}
			ts.wsService.SendTaskUpdate(taskID, result)
			continue
		}

		// 强制终止任务
		if err := runnerInstance.Kill(); err != nil {
			logger.Error("强制终止任务失败",
				zap.String("task_id", taskID),
				zap.Error(err))

			// 发送终止失败结果
			result := map[string]interface{}{
				"status":   core.TaskStatusError,
				"error":    fmt.Sprintf("强制终止任务失败: %v", err),
				"time_end": time.Now().Format("2006-01-02 15:04:05"),
			}
			ts.wsService.SendTaskUpdate(taskID, result)
		} else {
			logger.Info("任务强制终止请求已发送",
				zap.String("task_id", taskID))
		}
	}
}

// TimeoutTasks 处理超时任务列表
func (ts *taskServiceImpl) TimeoutTasks(tasks []*core.Task) {
	for _, task := range tasks {
		logger.Info("任务超时",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name))

		// 发送任务超时结果
		result := map[string]interface{}{
			"status": "timeout",
		}
		ts.wsService.SendTaskUpdate(task.ID.String(), result)
	}
}

// RetryTasks 重试任务列表
func (ts *taskServiceImpl) RetryTasks(tasks []*core.Task) {
	for _, task := range tasks {
		logger.Info("重试任务",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name))

		// 重新执行任务
		ts.ExecuteTask(task)
	}
}
