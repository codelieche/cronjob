// Package services 业务服务层
//
// 实现系统的核心业务逻辑，包括：
// - 任务调度服务：根据cron表达式创建和执行任务
// - WebSocket服务：与Worker节点进行实时通信
// - 分布式锁服务：确保任务不重复执行
// - 其他业务服务：用户、分类、工作节点等管理
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 全局任务队列定义
// 这些队列用于在API Server和Worker节点之间传递任务
var (
	// 待执行任务队列 - 存储等待Worker节点执行的任务
	// 容量为1024，超出容量时会阻塞或丢弃任务
	pendingTasksQueue = make(chan *core.Task, 1024)

	// 停止任务队列 - 存储需要停止执行的任务
	// 用于向Worker节点发送停止指令
	stopTasksQueue = make(chan *core.Task, 1024)
)

// NewDispatchService 创建任务调度服务实例
//
// 参数:
//   - cronJobStore: 定时任务数据存储接口
//   - taskStore: 任务记录数据存储接口
//   - locker: 分布式锁服务接口
//
// 返回值:
//   - core.DispatchService: 任务调度服务接口
func NewDispatchService(cronJobStore core.CronJobStore, taskStore core.TaskStore, locker core.Locker) core.DispatchService {
	return &DispatchService{
		cronJobStore: cronJobStore,
		taskStore:    taskStore,
		locker:       locker,
	}
}

// DispatchService 任务调度服务实现
//
// 负责系统的核心调度逻辑，包括：
// 1. 根据cron表达式创建任务实例
// 2. 管理任务的生命周期
// 3. 处理任务超时和重试
// 4. 与Worker节点协调任务执行
type DispatchService struct {
	cronJobStore core.CronJobStore // 定时任务数据存储
	taskStore    core.TaskStore    // 任务记录数据存储
	locker       core.Locker       // 分布式锁服务
	workflowExecService core.WorkflowExecuteService // 🔥 用于处理工作流任务超时
}

// Dispatch 调度cronjob
func (d *DispatchService) Dispatch(ctx context.Context, cronJob *core.CronJob) error {
	// 获取处理当前CronJob的锁，如果获取到了才继续，如果没有就跳过
	lockerKey := fmt.Sprintf(config.DispatchLockerKeyFormat, cronJob.ID.String())
	lockd, err := d.locker.Acquire(ctx, lockerKey, 10*time.Second)
	if err != nil {
		logger.Info("获取CronJob锁失败，跳过调度", zap.String("cronjob_id", cronJob.ID.String()), zap.Error(err))
		return nil
	} else {
		logger.Debug("获取到锁:" + lockerKey)
		defer lockd.Release(ctx)
	}

	// 获取当前时间
	now := time.Now()

	// 计算CronJob下次执行的时间作为LastPlan
	lastPlan, err := tools.GetNextExecutionTime(cronJob.Time, now)
	if err != nil {
		logger.Error("计算CronJob下次执行时间失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
		return err
	}

	// 查询数据库中是否有活跃状态的任务，且Task.TimeoutAt >= lastPlan
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "cronjob",
			Value:  cronJob.ID.String(),
			Op:     filters.FILTER_EQ,
		},
		// 🔥 只查询活跃状态的任务（pending、running），避免被已停止/取消的任务影响
		&filters.FilterOption{
			Column: "status",
			Value:  []string{core.TaskStatusPending, core.TaskStatusRunning},
			Op:     filters.FILTER_IN,
		},
		&filters.FilterOption{
			Column: "timeout_at",
			Value:  lastPlan.Format("2006-01-02 15:04:05"),
			Op:     filters.FILTER_GTE,
		},
	}

	tasks, err := d.taskStore.List(ctx, 0, 1, filterActions...)
	if err != nil {
		logger.Error("查询任务失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
		return err
	}

	// 如果没有符合条件的任务，则创建新任务
	if len(tasks) == 0 {
		// 创建Task对象
		isStandalone := false
		task := &core.Task{
			ID:           uuid.New(),
			TeamID:       cronJob.TeamID, // 继承CronJob的TeamID
			Project:      cronJob.Project,
			Category:     cronJob.Category,
			CronJob:      &cronJob.ID,
			Name:         cronJob.Name + "-" + lastPlan.Format("20060102-150405"),
			Command:      cronJob.Command,
			Args:         cronJob.Args,
			Description:  cronJob.Description,
			TimePlan:     lastPlan,
			Status:       core.TaskStatusPending,
			SaveLog:      cronJob.SaveLog,
			IsStandalone: &isStandalone,
			Timeout:      cronJob.Timeout,
			// 🔥 从CronJob继承重试配置
			MaxRetry:   cronJob.MaxRetry,
			Retryable:  cronJob.Retryable,
			RetryCount: 0, // 新任务重试次数为0
		}

		// 继承CronJob的元数据
		if err := task.InheritMetadataFromCronJob(cronJob, nil); err != nil {
			logger.Warn("继承CronJob元数据失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
		}

		// 计算TimeoutAt：基于LastPlan计算CronJob的再下一次执行时间
		timeoutAt, err := tools.GetNextExecutionTime(cronJob.Time, lastPlan)
		if err != nil {
			// 如果计算失败，设置为1小时后作为默认值
			timeoutAt = lastPlan.Add(1 * time.Hour)
			logger.Warn("计算任务超时时间失败，使用默认值", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
		}
		task.TimeoutAt = timeoutAt

		// 创建任务
		_, err = d.taskStore.Create(ctx, task)
		if err != nil {
			logger.Error("创建任务失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
			return err
		}

		// 更新CronJob的LastPlan
		cronJob.LastPlan = &lastPlan
		_, err = d.cronJobStore.Update(ctx, cronJob)
		if err != nil {
			logger.Error("更新CronJob失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
			return err
		}
		logger.Info("成功创建任务", zap.String("task_id", task.ID.String()), zap.String("cronjob_id", cronJob.ID.String()))
	}

	return nil
}

// DispatchLoop 循环调度CronJob，生产任务清单
func (d *DispatchService) DispatchLoop(ctx context.Context) error {
	logger.Info("开始运行调度循环")

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			logger.Info("调度循环被取消")
			return ctx.Err()
		default:
			// 继续执行
		}

		// 获取当前时间
		now := time.Now()
		// 🔥 保持1秒间隔，支持秒级调度
		nextExecuteTime := now.Add(1 * time.Second)

		// 🔥 P0优化：只查询真正需要调度的CronJob（last_plan <= now）
		// 同时提前预加载未来10秒内需要调度的（避免查询遗漏）
		// 这样既支持秒级调度，又大幅减少无效查询
		futureTime := now.Add(10 * time.Second)

		filterActions := []filters.Filter{
			&filters.FilterOption{
				Column: "is_active",
				Value:  true,
				Op:     filters.FILTER_EQ,
			},
			// 🔥 查询 last_plan <= now + 10秒（提前预加载）
			&filters.FilterOption{
				Column: "last_plan",
				Value:  futureTime.Format("2006-01-02 15:04:05"),
				Op:     filters.FILTER_LTE,
			},
		}

		// 🔥 LIMIT从100降到50（对于中小规模系统够用）
		cronJobs, err := d.cronJobStore.List(ctx, 0, 50, filterActions...)
		if err != nil {
			logger.Error("获取CronJob列表失败", zap.Error(err))
			time.Sleep(1 * time.Second) // 出错时暂停1秒后重试
			continue
		}

		// 🔥 只处理真正到期的cronjob（last_plan <= now）
		var needDispatchJobs []*core.CronJob
		for _, cronJob := range cronJobs {
			if cronJob.LastPlan != nil && (cronJob.LastPlan.Before(now) || cronJob.LastPlan.Equal(now)) {
				needDispatchJobs = append(needDispatchJobs, cronJob)
			}
		}

		if len(needDispatchJobs) > 0 {
			logger.Debug("发现需要调度的CronJob",
				zap.Int("total", len(cronJobs)),
				zap.Int("need_dispatch", len(needDispatchJobs)))
		}

		// 🔥 只遍历需要调度的CronJob
		for _, cronJob := range needDispatchJobs {
			// 在Dispatch中会获取锁，避免并发调度
			if err := d.Dispatch(ctx, cronJob); err != nil {
				logger.Error("调度CronJob失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
			}
		}

		// 计算等待时间
		waitDuration := time.Until(nextExecuteTime)
		if waitDuration > 0 {
			time.Sleep(waitDuration)
		} else {
			time.Sleep(10 * time.Millisecond) // 防止CPU空转
		}

		// 🔥 P0优化：移除NULL last_plan查询逻辑（减少86,400次/天无效查询）
		// 新建CronJob时应该在创建时就设置last_plan，不需要在循环中处理
		// 参考：pkg/services/cronjob.go的Create方法
	}
}

// CheckTaskLoop 检查任务是否过期
//
// 🔥 P0优化：拆分超时检查和待执行检查，使用不同的频率
// - 超时检查：每30秒一次（不紧急，减少98%查询）
// - 待执行检查：每3秒一次（保持响应性，减少83%查询）
func (d *DispatchService) CheckTaskLoop(ctx context.Context) error {
	logger.Info("开始运行任务检查循环")

	// 🔥 创建两个定时器
	timeoutTicker := time.NewTicker(30 * time.Second) // 超时检查：30秒
	pendingTicker := time.NewTicker(3 * time.Second)  // 待执行检查：3秒
	defer timeoutTicker.Stop()
	defer pendingTicker.Stop()

	// 🔥 立即执行一次检查
	d.checkTimeoutTasks(ctx)
	d.checkPendingTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("任务检查循环被取消")
			return ctx.Err()

		case <-timeoutTicker.C:
			// 🔥 每30秒检查一次超时任务
			d.checkTimeoutTasks(ctx)

		case <-pendingTicker.C:
			// 🔥 每3秒检查一次待执行任务
			d.checkPendingTasks(ctx)
		}
	}
}

// checkTimeoutTasks 检查并处理超时任务
func (d *DispatchService) checkTimeoutTasks(ctx context.Context) {
	now := time.Now()

	// 查询超时任务：Task.TimeoutAt <= now 且状态是pending
	timeoutFilter := []filters.Filter{
		&filters.FilterOption{
			Column: "timeout_at",
			Value:  now.Format("2006-01-02 15:04:05"),
			Op:     filters.FILTER_LTE,
		},
		&filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		},
	}

	// 🔥 LIMIT从100降到50
	timeoutTasks, err := d.taskStore.List(ctx, 0, 50, timeoutFilter...)
	if err != nil {
		logger.Error("获取超时任务失败", zap.Error(err))
		return
	}

	if len(timeoutTasks) > 0 {
		logger.Info("发现超时任务", zap.Int("count", len(timeoutTasks)))
	}

	// 处理超时任务
	for _, task := range timeoutTasks {
		func(task *core.Task) {
			// 获取任务锁
			lockKey := fmt.Sprintf(config.TaskLockerKeyFormat, task.ID.String())
			lockd, err := d.locker.Acquire(ctx, lockKey, 100*time.Second)
			if err != nil {
				logger.Debug("获取任务锁失败，跳过处理",
					zap.String("task_id", task.ID.String()),
					zap.Error(err))
				return
			}
			defer lockd.Release(ctx)

			// 更新任务状态为timeout
			task.Status = core.TaskStatusTimeout
			task.TimeEnd = &now

			// 更新任务
			_, err = d.taskStore.Update(ctx, task)
			if err != nil {
				logger.Error("更新超时任务失败",
					zap.Error(err),
					zap.String("task_id", task.ID.String()))
				return
			}

			logger.Info("任务已超时",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))

			// 🔥 如果是工作流任务，触发 HandleTaskComplete
			if task.IsWorkflowTask() && d.workflowExecService != nil {
				// 异步调用，避免阻塞调度循环
				go func(taskID uuid.UUID) {
					if err := d.workflowExecService.HandleTaskComplete(context.Background(), taskID); err != nil {
						logger.Error("处理超时工作流任务失败",
							zap.Error(err),
							zap.String("task_id", taskID.String()))
					}
				}(task.ID)
			}
		}(task)
	}
}

// checkPendingTasks 检查并分发待执行任务
func (d *DispatchService) checkPendingTasks(ctx context.Context) {
	now := time.Now()

	// 查询待处理任务：Task.TimePlan <= now < Task.TimeoutAt 且状态是Pending
	pendingFilter := []filters.Filter{
		&filters.FilterOption{
			Column: "time_plan",
			Value:  now,
			Op:     filters.FILTER_LTE,
		},
		&filters.FilterOption{
			Column: "timeout_at",
			Value:  now,
			Op:     filters.FILTER_GT,
		},
		&filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		},
	}

	// 🔥 LIMIT从100降到50
	pendingTasks, err := d.taskStore.List(ctx, 0, 50, pendingFilter...)
	if err != nil {
		logger.Error("获取待处理任务失败", zap.Error(err))
		return
	}

	if len(pendingTasks) > 0 {
		logger.Info("发现待执行任务", zap.Int("count", len(pendingTasks)))
	}

	// 将待处理任务加入全局队列
	for _, task := range pendingTasks {
		select {
		case pendingTasksQueue <- task:
			logger.Debug("任务已加入队列",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))
		default:
			// 队列已满，记录警告
			logger.Warn("待处理任务队列已满",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))
		}
	}
}

// Stop 停止任务
func (d *DispatchService) Stop(ctx context.Context, task *core.Task) error {
	// 将任务加入停止队列
	select {
	case stopTasksQueue <- task:
		logger.Info("任务已加入停止队列", zap.String("task_id", task.ID.String()))
		return nil
	default:
		// 队列已满，返回错误
		err := errors.New("停止任务队列已满，无法添加新任务")
		logger.Error("停止任务队列已满", zap.String("task_id", task.ID.String()))
		return err
	}
}

// GetPendingTasks 获取待执行任务列表
func (d *DispatchService) GetPendingTasks(ctx context.Context) ([]*core.Task, error) {
	// 获取当前时间
	now := time.Now()

	// 构建过滤器：Task.TimePlan <= now < Task.TimeoutAt 且状态是Pending
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "time_plan",
			Value:  now,
			Op:     filters.FILTER_LTE,
		},
		&filters.FilterOption{
			Column: "timeout_at",
			Value:  now,
			Op:     filters.FILTER_GT,
		},
		&filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		},
	}

	// 从数据库获取待处理任务
	tasks, err := d.taskStore.List(ctx, 0, 1000, filterActions...)
	if err != nil {
		logger.Error("获取待处理任务失败", zap.Error(err))
		return nil, err
	}

	logger.Info("成功获取待处理任务列表", zap.Int("count", len(tasks)))
	return tasks, nil
}

// 获取全局队列 - 供外部使用的辅助函数
func GetPendingTasksQueue() <-chan *core.Task {
	return pendingTasksQueue
}

func GetStopTasksQueue() <-chan *core.Task {
	return stopTasksQueue
}

// ============================================
// 🔥 任务自动重试功能
// ============================================

// CheckFailedTasksLoop 检查失败任务并自动重试
//
// 定期检查失败的任务，并根据重试策略创建新的重试任务
// 重试策略：
// - 只重试标记为可重试的任务（retryable=true）
// - 重试次数未达到最大限制（retry_count < max_retry）
// - 已到重试时间（next_retry_time <= now）
//
// 重试逻辑：
// - 创建新的Task对象（状态为pending）
// - 递增retry_count
// - 保留原Task配置（max_retry, retryable等）
//
// 参数:
//   - ctx: 上下文对象
//
// 返回:
//   - error: 错误信息
func (d *DispatchService) CheckFailedTasksLoop(ctx context.Context) error {
	// 检查是否启用自动重试
	if !config.Retry.Enabled {
		logger.Info("自动重试功能已禁用")
		return nil
	}

	logger.Info("启动失败任务检查循环（立即重试策略）",
		zap.Duration("check_interval", config.Retry.CheckInterval))

	ticker := time.NewTicker(config.Retry.CheckInterval)
	defer ticker.Stop()

	// 立即执行一次
	d.checkFailedTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("失败任务检查循环已停止")
			return ctx.Err()
		case <-ticker.C:
			d.checkFailedTasks(ctx)
		}
	}
}

// checkFailedTasks 检查失败任务并触发重试
//
// 🔥 设计思路：
//  1. 只有 is_retry=false 的原始任务可以被重试（重试任务不可再重试）
//  2. 不重试 timeout 任务（新调度周期会产生新任务）
//  3. 创建重试任务时：将原任务的 next_retry_time 设置为 NULL（正在重试中）
//  4. 重试任务失败时：将原任务的 next_retry_time 设置为 NOW（等待下次检查）
//  5. 重试任务成功时：将原任务的 retryable 设置为 false（停止重试）
//
// 🔒 全局锁：
//   - 在多副本环境中，只有一个副本可以处理重试逻辑
//   - 获取不到锁则跳过（说明其他副本正在处理）
//   - 防止同一个失败任务被多次重试
//
// 📌 查询条件：
//   - status IN (failed, error) - 不包括timeout
//   - is_retry = false - 不是重试任务
//   - retryable = true - 可重试
//   - next_retry_time IS NOT NULL AND <= now - 已到重试时间
//   - retry_count < max_retry - 未达到最大重试次数
//   - max_retry > 0 - 配置了重试
func (d *DispatchService) checkFailedTasks(ctx context.Context) {
	// 🔒 尝试获取全局锁（多副本环境下只有一个副本可以处理重试）
	lockKey := "cronjob:retry:check-failed-tasks"
	lockd, err := d.locker.Acquire(ctx, lockKey, 30*time.Second)
	if err != nil {
		logger.Debug("获取重试检查全局锁失败，跳过（其他副本正在处理）",
			zap.String("lock_key", lockKey),
			zap.Error(err))
		return
	}
	defer lockd.Release(ctx)

	logger.Debug("已获取重试检查全局锁，开始处理",
		zap.String("lock_key", lockKey))

	now := time.Now()

	// 🔥 使用专门的查询方法，职责清晰
	tasks, err := d.taskStore.GetNeedRetryTasks(ctx, 1000)
	if err != nil {
		logger.Error("查询需要重试的任务失败", zap.Error(err))
		return
	}

	if len(tasks) == 0 {
		logger.Debug("没有需要重试的失败任务")
		return
	}

	logger.Info("发现需要重试的失败任务",
		zap.Int("count", len(tasks)),
		zap.Time("check_time", now))

	// 逐个处理任务
	successCount := 0
	failCount := 0
	skipCount := 0

	for _, task := range tasks {
		// 🔥 使用 ShouldRetry 进行更严格的检查（包括超时检查）
		if !core.ShouldRetry(task) {
			logger.Debug("任务不应该重试，跳过",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name),
				zap.Int("retry_count", task.RetryCount),
				zap.Int("max_retry", task.MaxRetry),
				zap.String("status", task.Status),
				zap.Time("timeout_at", task.TimeoutAt))
			skipCount++
			continue
		}

		// 尝试重试任务
		if err := d.retryTask(ctx, task); err != nil {
			logger.Error("重试任务失败",
				zap.Error(err),
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name))
			failCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 || failCount > 0 || skipCount > 0 {
		logger.Info("失败任务检查完成",
			zap.Int("total", len(tasks)),
			zap.Int("success", successCount),
			zap.Int("fail", failCount),
			zap.Int("skip", skipCount))
	}
}

// retryTask 重试任务
//
// 创建一个新的Task对象来重试失败的任务：
// 1. 复制原任务的配置（command, args, timeout等）
// 2. 递增retry_count
// 3. 设置状态为pending
// 4. 计算新的next_retry_time（为下次可能的重试做准备）
//
// 参数:
//   - ctx: 上下文对象
//   - task: 失败的任务
//
// 返回:
//   - error: 错误信息
func (d *DispatchService) retryTask(ctx context.Context, task *core.Task) error {
	// 🔒 获取单个任务锁（双重保险）
	// 虽然已有全局锁保证只有一个副本在处理重试检查，
	// 但单个任务锁可以防止同一任务被并发重试（额外的安全措施）
	lockKey := fmt.Sprintf("task:retry:%s", task.ID.String())
	lockd, err := d.locker.Acquire(ctx, lockKey, 30*time.Second)
	if err != nil {
		logger.Warn("获取任务重试锁失败，跳过",
			zap.String("task_id", task.ID.String()),
			zap.Error(err))
		return nil // 不返回错误，避免影响其他任务
	}
	defer lockd.Release(ctx)

	// 重新查询任务状态（确保状态一致）
	currentTask, err := d.taskStore.FindByID(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("查询任务失败: %w", err)
	}

	// 再次检查是否应该重试（防止并发问题）
	if !core.ShouldRetry(currentTask) {
		// 🔥 判断具体原因，提供更详细的日志
		reason := "unknown"
		if currentTask.Retryable == nil || !*currentTask.Retryable {
			reason = "retryable=false"
		} else if currentTask.RetryCount >= currentTask.MaxRetry {
			reason = "max_retry_reached"
		} else if !currentTask.TimeoutAt.IsZero() {
			now := time.Now()
			if now.Sub(currentTask.TimeoutAt) > 30*time.Minute {
				reason = "timeout_exceeded_grace_period"
			}
		}

		logger.Debug("任务不应该重试，跳过",
			zap.String("task_id", task.ID.String()),
			zap.String("reason", reason),
			zap.String("status", currentTask.Status),
			zap.Int("retry_count", currentTask.RetryCount),
			zap.Int("max_retry", currentTask.MaxRetry),
			zap.Time("timeout_at", currentTask.TimeoutAt))
		return nil
	}

	// 创建新任务（重试）
	now := time.Now()
	newTaskID := uuid.New()
	newRetryCount := currentTask.RetryCount + 1

	// 🔥 重试任务不应该再被重试，直接设置 retryable=false
	retryable := false
	isRetry := true

	// 🔥 在Metadata中设置parent_task字段
	var metadata map[string]interface{}
	if len(currentTask.Metadata) > 0 {
		// 解析现有Metadata
		if err := json.Unmarshal(currentTask.Metadata, &metadata); err != nil {
			logger.Warn("解析任务Metadata失败，创建新的", zap.Error(err))
			metadata = make(map[string]interface{})
		}
	} else {
		metadata = make(map[string]interface{})
	}

	// 🔥 设置parent_task字段
	metadata["parent_task"] = currentTask.ID.String()

	// 重新序列化Metadata
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		logger.Error("序列化Metadata失败", zap.Error(err))
		return err
	}

	newTask := &core.Task{
		ID:            newTaskID,
		TeamID:        currentTask.TeamID,
		Project:       currentTask.Project,
		Category:      currentTask.Category,
		CronJob:       currentTask.CronJob,
		Name:          fmt.Sprintf("%s-retry-%d", currentTask.Name, newRetryCount),
		Command:       currentTask.Command,
		Args:          currentTask.Args,
		Description:   fmt.Sprintf("重试任务 (第%d次重试)", newRetryCount),
		TimePlan:      now,
		TimeoutAt:     currentTask.TimeoutAt, // 🔥 继承原任务的 TimeoutAt（重要！）
		Status:        core.TaskStatusPending,
		SaveLog:       currentTask.SaveLog,
		RetryCount:    newRetryCount,        // 🔥 递增重试计数
		MaxRetry:      currentTask.MaxRetry, // 🔥 复制最大重试次数
		Retryable:     &retryable,           // 🔥 重试任务不可再重试
		NextRetryTime: nil,                  // 🔥 不需要设置下次重试时间
		IsRetry:       &isRetry,             // 🔥 标记为重试任务
		Timeout:       currentTask.Timeout,
		Metadata:      metadataBytes, // 🔥 包含parent_task的Metadata
		IsStandalone:  currentTask.IsStandalone,
	}

	// 创建新任务
	createdTask, err := d.taskStore.Create(ctx, newTask)
	if err != nil {
		logger.Error("创建重试任务失败", zap.Error(err))
		return err
	}

	// 🔥 更新原任务的 retry_count、next_retry_time 和 metadata
	// 注意：不设置 retryable=false，保持可重试状态，直到重试成功

	// 解析原任务的Metadata
	var originalMetadata map[string]interface{}
	if len(currentTask.Metadata) > 0 {
		if err := json.Unmarshal(currentTask.Metadata, &originalMetadata); err != nil {
			logger.Warn("解析原任务Metadata失败，创建新的", zap.Error(err))
			originalMetadata = make(map[string]interface{})
		}
	} else {
		originalMetadata = make(map[string]interface{})
	}

	// 🔥 将重试任务ID添加到retry_tasks数组中
	var retryTasks []string
	if existing, ok := originalMetadata["retry_tasks"].([]interface{}); ok {
		// 转换已有的retry_tasks
		for _, t := range existing {
			if taskID, ok := t.(string); ok {
				retryTasks = append(retryTasks, taskID)
			}
		}
	}
	// 添加新的重试任务ID
	retryTasks = append(retryTasks, newTaskID.String())
	originalMetadata["retry_tasks"] = retryTasks

	// 重新序列化Metadata
	updatedMetadata, err := json.Marshal(originalMetadata)
	if err != nil {
		logger.Error("序列化原任务Metadata失败", zap.Error(err))
		// 继续执行，不影响其他字段更新
	}

	// 🔥 更新原任务状态
	// 1. 递增 retry_count
	// 2. 将 next_retry_time 设置为 NULL（表示正在重试中）
	// 3. 如果达到最大重试次数，设置 retryable=false
	updates := map[string]interface{}{
		"retry_count":     newRetryCount, // 更新重试计数
		"next_retry_time": nil,           // 🔥 置空，表示正在重试中
	}

	// 如果达到最大重试次数，设置 retryable=false
	if newRetryCount >= currentTask.MaxRetry {
		falseValue := false
		updates["retryable"] = &falseValue
		logger.Info("已达到最大重试次数，此次重试为最后一次",
			zap.String("task_id", currentTask.ID.String()),
			zap.Int("retry_count", newRetryCount),
			zap.Int("max_retry", currentTask.MaxRetry))
	}

	// 只有Metadata更新成功才添加到updates中
	if updatedMetadata != nil {
		updates["metadata"] = updatedMetadata
	}

	if err := d.taskStore.Patch(ctx, currentTask.ID, updates); err != nil {
		logger.Warn("更新原任务重试状态失败",
			zap.Error(err),
			zap.String("task_id", currentTask.ID.String()))
		// 不返回错误，因为重试任务已经创建成功
	} else {
		logger.Debug("已更新原任务，next_retry_time置空（正在重试中）",
			zap.String("original_task_id", currentTask.ID.String()),
			zap.String("retry_task_id", newTaskID.String()),
			zap.Int("retry_count", newRetryCount),
			zap.Strings("retry_tasks", retryTasks))
	}

	logger.Info("重试任务已创建",
		zap.String("original_task_id", task.ID.String()),
		zap.String("original_task_name", task.Name),
		zap.String("new_task_id", createdTask.ID.String()),
		zap.String("new_task_name", createdTask.Name),
		zap.Int("retry_count", newRetryCount),
		zap.Int("max_retry", createdTask.MaxRetry))

	return nil
}

// RetryTask 手动重试任务（API调用）
//
// 用于用户手动触发任务重试，与自动重试不同：
// - 不检查next_retry_time（立即重试）
// - 仍然检查retry_count和retryable
//
// 参数:
//   - ctx: 上下文对象
//   - taskID: 任务ID
//
// 返回:
//   - *core.Task: 新创建的重试任务
//   - error: 错误信息
func (d *DispatchService) RetryTask(ctx context.Context, taskID string) (*core.Task, error) {
	// 解析taskID
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("无效的任务ID: %w", err)
	}

	// 查询原任务
	task, err := d.taskStore.FindByID(ctx, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}

	// 检查任务状态
	if !core.ShouldRetry(task) {
		return nil, fmt.Errorf("任务不满足重试条件：status=%s, retryable=%v, retry_count=%d, max_retry=%d",
			task.Status, task.Retryable, task.RetryCount, task.MaxRetry)
	}

	// 调用内部重试逻辑
	if err := d.retryTask(ctx, task); err != nil {
		return nil, err
	}

	// 查询新创建的重试任务
	newTaskName := fmt.Sprintf("%s-retry-%d", task.Name, task.RetryCount+1)
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "name",
			Value:  newTaskName,
			Op:     filters.FILTER_EQ,
		},
	}
	tasks, err := d.taskStore.List(ctx, 0, 1, filterActions...)
	if err != nil || len(tasks) == 0 {
		return nil, fmt.Errorf("查询新创建的重试任务失败")
	}

	return tasks[0], nil
}

// SetWorkflowExecuteService 设置工作流执行服务（用于依赖注入）
func (d *DispatchService) SetWorkflowExecuteService(service core.WorkflowExecuteService) {
	d.workflowExecService = service
}
