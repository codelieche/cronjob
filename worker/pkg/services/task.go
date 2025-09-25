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

// TaskServiceImpl 任务执行服务实现
//
// 负责处理从API Server接收到的任务，包括：
// - 任务执行逻辑
// - 状态更新和结果上报
// - 分布式锁管理
// - 任务重试和超时处理
type TaskServiceImpl struct {
	updateCallback core.TaskUpdateCallback // 任务更新回调，用于发送状态更新（解决循环依赖）
	errorHandler   core.ErrorHandler       // 统一错误处理器
	locker         core.Locker             // 分布式锁服务，确保任务不重复执行
	apiserver      core.Apiserver          // API Server通信服务，用于获取任务详情
	runningTasks   map[string]core.Runner  // 正在运行的任务，key为task_id
	taskMutex      sync.RWMutex            // 保护runningTasks的并发访问
}

// NewTaskService 创建任务执行服务实例
//
// 参数:
//   - updateCallback: 任务更新回调，用于发送状态更新
//   - apiserver: API Server通信服务
//
// 返回值:
//   - core.TaskService: 任务执行服务接口
func NewTaskService(updateCallback core.TaskUpdateCallback, apiserver core.Apiserver) core.TaskService {
	return &TaskServiceImpl{
		updateCallback: updateCallback,                       // 任务更新回调（解决循环依赖）
		errorHandler:   core.NewErrorHandler(updateCallback), // 创建统一错误处理器
		locker:         NewLocker(),                          // 创建分布式锁服务实例
		apiserver:      apiserver,                            // API Server通信服务，用于获取任务详情
		runningTasks:   make(map[string]core.Runner),         // 初始化正在运行的任务映射
	}
}

// HandleTaskEvent 处理任务事件
//
// 根据事件类型分发到相应的处理方法
// 支持的事件类型：运行、停止、终止、超时、重试
// 在处理任务前会检查WorkerSelect配置，确保当前Worker有权限执行任务
//
// 参数:
//   - event: 任务事件对象，包含事件类型和任务列表
func (ts *TaskServiceImpl) HandleTaskEvent(event *core.TaskEvent) {
	logger.Info("收到任务事件", zap.String("action", event.Action), zap.Int("task_count", len(event.Tasks)))

	// 过滤任务：检查WorkerSelect配置
	filteredTasks := ts.filterTasksByWorkerSelect(event.Tasks)
	if len(filteredTasks) == 0 {
		logger.Info("没有适合当前Worker执行的任务", zap.Int("original_count", len(event.Tasks)))
		return
	}

	if len(filteredTasks) < len(event.Tasks) {
		logger.Info("部分任务被WorkerSelect过滤",
			zap.Int("original_count", len(event.Tasks)),
			zap.Int("filtered_count", len(filteredTasks)))
	}

	switch event.Action {
	case string(core.TaskActionRun):
		ts.RunTasks(filteredTasks) // 运行任务
	case string(core.TaskActionStop):
		ts.StopTasks(filteredTasks) // 停止任务
	case string(core.TaskActionKill):
		ts.KillTasks(filteredTasks) // 强制终止任务
	case string(core.TaskActionTimeout):
		ts.TimeoutTasks(filteredTasks) // 处理超时任务
	case string(core.TaskActionRetry):
		ts.RetryTasks(filteredTasks) // 重试任务
	default:
		logger.Warn("未知的任务事件类型", zap.String("action", event.Action))
	}
}

// filterTasksByWorkerSelect 根据WorkerSelect配置过滤任务
//
// 检查任务的元数据中是否配置了WorkerSelect，如果配置了则检查当前Worker是否在允许列表中
// 如果没有配置WorkerSelect或当前Worker在允许列表中，则返回该任务
//
// 参数:
//   - tasks: 原始任务列表
//
// 返回值:
//   - []*core.Task: 过滤后的任务列表
func (ts *TaskServiceImpl) filterTasksByWorkerSelect(tasks []*core.Task) []*core.Task {
	var filteredTasks []*core.Task

	for _, task := range tasks {
		// 检查任务是否有元数据配置
		if len(task.Metadata) == 0 {
			// 没有元数据配置，允许执行
			filteredTasks = append(filteredTasks, task)
			continue
		}

		// 解析任务元数据
		taskMetadata, err := task.GetMetadata()
		if err != nil {
			logger.Warn("解析任务元数据失败，跳过WorkerSelect检查",
				zap.Error(err),
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))
			// 解析失败时，允许执行（保持兼容性）
			filteredTasks = append(filteredTasks, task)
			continue
		}

		// 检查WorkerSelect配置
		if len(taskMetadata.WorkerSelect) == 0 {
			// 没有配置WorkerSelect，表示所有Worker都可以执行
			filteredTasks = append(filteredTasks, task)
			continue
		}

		// 检查当前Worker是否在允许列表中
		currentWorkerID := config.WorkerInstance.ID.String()
		currentWorkerName := config.WorkerInstance.Name

		workerSelected := false
		for _, selectedWorker := range taskMetadata.WorkerSelect {
			// 支持按Worker ID或Name进行匹配
			if selectedWorker == currentWorkerID || selectedWorker == currentWorkerName {
				workerSelected = true
				break
			}
		}

		if workerSelected {
			// 当前Worker在允许列表中，可以执行
			filteredTasks = append(filteredTasks, task)
			logger.Debug("任务通过WorkerSelect检查",
				zap.String("task_id", task.ID.String()),
				zap.String("worker_id", currentWorkerID),
				zap.String("worker_name", currentWorkerName),
				zap.Strings("worker_select", taskMetadata.WorkerSelect))
		} else {
			// 当前Worker不在允许列表中，跳过执行
			logger.Info("任务指定了WorkerSelect，当前Worker不在允许列表中",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name),
				zap.String("worker_id", currentWorkerID),
				zap.String("worker_name", currentWorkerName),
				zap.Strings("worker_select", taskMetadata.WorkerSelect))
		}
	}

	return filteredTasks
}

// ExecuteTask 执行任务
//
// 检查任务类型是否支持，如果支持则异步执行任务
// 使用分布式锁确保同一任务不会被多个Worker同时执行
//
// 参数:
//   - task: 要执行的任务对象
func (ts *TaskServiceImpl) ExecuteTask(task *core.Task) {
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
func (ts *TaskServiceImpl) executeTask(task *core.Task) {
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
	ts.updateCallback.SendTaskUpdate(task.ID.String(), taskStart)

	// 创建并执行任务
	var taskResult *core.Result

	// 根据任务类型创建相应的Runner
	runnerInstance, err := core.CreateRunner(task.Category)
	if err != nil {
		ts.handleTaskError(ctx, err, task, "CreateRunner")
		return
	}

	// 解析任务参数和配置
	if err := runnerInstance.ParseArgs(task); err != nil {
		ts.handleTaskError(ctx, err, task, "ParseArgs")
		runnerInstance.Cleanup()
		return
	}

	logger.Debug("成功解析任务配置",
		zap.String("task_id", task.ID.String()),
		zap.String("command", task.Command),
		zap.String("args", task.Args),
		zap.Int("timeout", task.Timeout))

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

	// 根据task.SaveLog决定是否启用实时日志回写
	var logChan chan string
	if task.SaveLog != nil && *task.SaveLog {
		// 创建日志通道用于实时日志回写
		logChan = make(chan string, 100) // 阻塞，确保日志顺序
		defer close(logChan)

		// 启动goroutine处理实时日志回写
		go ts.handleRealtimeLogs(task.ID.String(), logChan)
	}

	// 执行任务
	logger.Info("开始执行任务",
		zap.String("task_id", task.ID.String()),
		zap.String("category", task.Category),
		zap.String("command", task.Command),
		zap.Int("timeout", task.Timeout),
		zap.Bool("save_log", task.SaveLog != nil && *task.SaveLog))

	// 执行任务：核心功能
	taskResult, err = runnerInstance.Execute(ctx, logChan)

	// 处理执行结果
	if err != nil {
		ts.handleTaskError(ctx, err, task, "ExecuteTask")
		return
	}

	// 处理Runner返回的结果
	if taskResult == nil {
		ts.handleTaskError(ctx, fmt.Errorf("任务执行结果为空"), task, "ExecuteTask")
		return
	}

	// 根据Runner结果状态映射到Task状态
	var taskStatus string
	switch taskResult.Status {
	case core.StatusSuccess:
		taskStatus = core.TaskStatusSuccess
		if task.SaveLog == nil || !*task.SaveLog {
			// 无需保存日志的话，就将输出设置为执行成功
			taskResult.Output = "执行成功"
		}
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

	// 添加输出信息（用于后续任务取数据）
	if taskResult.Output != "" {
		result["output"] = taskResult.Output
	}

	// 添加执行日志（用于显示给用户）
	if taskResult.ExecuteLog != "" {
		result["execute_log"] = taskResult.ExecuteLog
	}

	if taskStatus != core.TaskStatusSuccess {
		// 任务失败，添加错误信息
		if taskResult.Error != "" {
			if executeLog, ok := result["execute_log"].(string); ok {
				result["execute_log"] = executeLog + "\n\n-------\n\n" + taskResult.Error
			} else {
				result["execute_log"] = taskResult.Error
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
	ts.updateCallback.SendTaskUpdate(task.ID.String(), result)

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
func (ts *TaskServiceImpl) isTaskCategorySupported(category string) bool {
	supportedTasks := config.WorkerInstance.Metadata.Tasks
	for _, supportedCategory := range supportedTasks {
		if supportedCategory == category {
			return true
		}
	}
	return false
}

// 注意：sendErrorResult方法已被移除，统一使用ErrorHandler处理错误

// handleRealtimeLogs 处理实时日志回写
func (ts *TaskServiceImpl) handleRealtimeLogs(taskID string, logChan <-chan string) {

	// 收集所有日志内容
	// var allLogs strings.Builder

	for logContent := range logChan {
		// 收到空消息就是退出：日志消息写完了
		if logContent == "" {
			break
		}
		// logger.Info("收到实时日志", zap.String("task_id", taskID), zap.String("log_content", logContent))
		if logContent != "" {
			// 追加到本地日志收集器
			// allLogs.WriteString(logContent)

			// 实时回写到API Server
			if err := ts.apiserver.AppendTaskLog(taskID, logContent); err != nil {
				logger.Error("回写任务日志失败",
					zap.String("task_id", taskID),
					zap.Error(err))
			}
		}
	}
	// 执行结果的日志

	// logger.Info("实时日志处理完成",
	// 	zap.String("task_id", taskID),
	// 	zap.Int("total_log_size", allLogs.Len()))
}

// RunTasks 运行任务列表
func (ts *TaskServiceImpl) RunTasks(tasks []*core.Task) {
	for _, task := range tasks {
		ts.ExecuteTask(task)
	}
}

// StopTasks 停止任务列表
func (ts *TaskServiceImpl) StopTasks(tasks []*core.Task) {
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
			ts.updateCallback.SendTaskUpdate(taskID, result)
			continue
		}

		// 尝试优雅停止任务
		if err := runnerInstance.Stop(); err != nil {
			ts.handleTaskError(context.Background(), err, task, "StopTask")
		} else {
			logger.Info("任务停止请求已发送",
				zap.String("task_id", taskID))
		}
	}
}

// KillTasks 强制终止任务列表
func (ts *TaskServiceImpl) KillTasks(tasks []*core.Task) {
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
			ts.updateCallback.SendTaskUpdate(taskID, result)
			continue
		}

		// 强制终止任务
		if err := runnerInstance.Kill(); err != nil {
			ts.handleTaskError(context.Background(), err, task, "KillTask")
		} else {
			logger.Info("任务强制终止请求已发送",
				zap.String("task_id", taskID))
		}
	}
}

// TimeoutTasks 处理超时任务列表
func (ts *TaskServiceImpl) TimeoutTasks(tasks []*core.Task) {
	for _, task := range tasks {
		logger.Info("任务超时",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name))

		// 发送任务超时结果
		result := map[string]interface{}{
			"status": "timeout",
		}
		ts.updateCallback.SendTaskUpdate(task.ID.String(), result)
	}
}

// RetryTasks 重试任务列表
func (ts *TaskServiceImpl) RetryTasks(tasks []*core.Task) {
	for _, task := range tasks {
		logger.Info("重试任务",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name))

		// 重新执行任务
		ts.ExecuteTask(task)
	}
}

// GetRunningTaskCount 获取正在运行的任务数量
func (ts *TaskServiceImpl) GetRunningTaskCount() int {
	ts.taskMutex.RLock()
	defer ts.taskMutex.RUnlock()
	return len(ts.runningTasks)
}

// GetRunningTaskIDs 获取正在运行的任务ID列表
func (ts *TaskServiceImpl) GetRunningTaskIDs() []string {
	ts.taskMutex.RLock()
	defer ts.taskMutex.RUnlock()

	taskIDs := make([]string, 0, len(ts.runningTasks))
	for taskID := range ts.runningTasks {
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

// WaitForTasksCompletion 等待所有任务完成
func (ts *TaskServiceImpl) WaitForTasksCompletion(timeout time.Duration) error {
	logger.Info("等待所有任务完成", zap.Duration("timeout", timeout))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			runningCount := ts.GetRunningTaskCount()
			runningIDs := ts.GetRunningTaskIDs()
			logger.Warn("等待任务完成超时",
				zap.Int("remaining_tasks", runningCount),
				zap.Strings("task_ids", runningIDs))
			return fmt.Errorf("等待任务完成超时，还有 %d 个任务未完成", runningCount)
		case <-ticker.C:
			runningCount := ts.GetRunningTaskCount()
			if runningCount == 0 {
				logger.Info("所有任务已完成")
				return nil
			}
			logger.Debug("等待任务完成", zap.Int("remaining_tasks", runningCount))
		}
	}
}

// handleTaskError 处理任务错误的辅助函数
//
// 参数:
//   - ctx: 上下文信息
//   - err: 错误对象
//   - task: 任务对象
//   - action: 执行的动作名称
//
// 功能:
//   - 统一错误处理格式
//   - 自动提取任务相关信息
//   - 简化错误处理代码
func (ts *TaskServiceImpl) handleTaskError(ctx context.Context, err error, task *core.Task, action string) {
	ts.errorHandler.HandleTaskError(ctx, err, core.ErrorContext{
		TaskID:    task.ID.String(),
		Component: "TaskService",
		Action:    action,
		Level:     core.LevelError,
		Extra: map[string]interface{}{
			"category": task.Category,
			"name":     task.Name,
		},
	})
}
