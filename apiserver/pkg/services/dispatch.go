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

	// 查询数据库中是否有非Pending的任务，且Task.TimeoutAt小于等于lastPlan
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "cronjob",
			Value:  cronJob.ID.String(),
			Op:     filters.FILTER_EQ,
		},
		// &filters.FilterOption{
		// 	Column: "status",
		// 	Value:  core.TaskStatusPending,
		// 	Op:     filters.FILTER_NEQ,
		// },
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
		// 计算下一次执行时间（now+1秒）
		nextExecuteTime := now.Add(1 * time.Second)

		// 获取所有DispatchLoop值小于等于当前时间的CronJob列表
		// 注意：这里的逻辑可能需要调整，因为CronJob结构体中没有DispatchLoop字段
		// 我们假设需要获取所有激活的CronJob
		filterActions := []filters.Filter{
			&filters.FilterOption{
				Column: "is_active",
				Value:  true,
				Op:     filters.FILTER_EQ,
			},
			// 如果是空的，获取不到，那么会持续的无法调度，那么我们可以在创建的时候，设置当前值为last_plan
			&filters.FilterOption{
				Column: "last_plan",
				Value:  now.Format("2006-01-02 15:04:05"),
				Op:     filters.FILTER_LTE,
			},
		}

		cronJobs, err := d.cronJobStore.List(ctx, 0, 100, filterActions...)
		if err != nil {
			logger.Error("获取CronJob列表失败", zap.Error(err))
			time.Sleep(1 * time.Second) // 出错时暂停1秒后重试
			continue
		}

		// 遍历CronJob列表，调用Dispatch方法
		for _, cronJob := range cronJobs {
			// 在Dispatch中会获取锁，避免并发调度

			// 调用Dispatch方法
			if err := d.Dispatch(ctx, cronJob); err != nil {
				logger.Error("调度CronJob失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
			}

			// 释放锁
		}

		// 计算等待时间
		waitDuration := time.Until(nextExecuteTime)
		if waitDuration > 0 {
			time.Sleep(waitDuration)
		} else {
			time.Sleep(10 * time.Millisecond) // 防止CPU空转
		}

		filterNullActions := []filters.Filter{
			&filters.FilterOption{
				Column: "is_active",
				Value:  true,
				Op:     filters.FILTER_EQ,
			},
			// 如果是空的，获取不到，那么会持续的无法调度，那么我们可以在创建的时候，设置当前值为last_plan
			&filters.FilterOption{
				Column:    "last_plan",
				Value:     nil,
				Op:        filters.FILTER_EQ,
				AllowNull: true, // 允许parseExpression处理NULL值
			},
		}
		// 得到PlanTime是NULL的CronJob列表,我们手动更新这条，后面其实可以让Store实现一个批量更新NULL为当前时间的函数
		cronNullPlanTimeJobs, _ := d.cronJobStore.List(ctx, 0, 100, filterNullActions...)
		if cronNullPlanTimeJobs == nil {
			continue
		}
		// 遍历CronJob列表，调用Dispatch方法
		for _, cronJob := range cronNullPlanTimeJobs {
			cronJob.LastPlan = &now
			_, err = d.cronJobStore.Update(ctx, cronJob)
			if err != nil {
				logger.Error("更新CronJob失败", zap.Error(err), zap.String("cronjob_id", cronJob.ID.String()))
				continue
			}
		}
	}
}

// CheckTaskLoop 检查任务是否过期
func (d *DispatchService) CheckTaskLoop(ctx context.Context) error {
	logger.Info("开始运行任务检查循环")

	for {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			logger.Info("任务检查循环被取消")
			return ctx.Err()
		default:
			// 继续执行
		}

		// 获取当前时间
		now := time.Now()

		// 查询超时任务：Task.TimeoutAt <= now 且状态不是timeout
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

		timeoutTasks, err := d.taskStore.List(ctx, 0, 100, timeoutFilter...)
		if err != nil {
			logger.Error("获取超时任务失败", zap.Error(err))
			time.Sleep(1 * time.Second) // 出错时暂停1秒后重试
			continue
		}

		// 处理超时任务
		for _, task := range timeoutTasks {
			func(task *core.Task) {
				// 获取任务锁
				lockKey := fmt.Sprintf(config.TaskLockerKeyFormat, task.ID.String())
				lockd, err := d.locker.Acquire(ctx, lockKey, 100*time.Second)
				if err != nil {
					logger.Info("获取任务锁失败，跳过处理", zap.String("task_id", task.ID.String()), zap.Error(err))
					return
				} else {
					logger.Debug("获取到锁:" + lockKey)
					defer lockd.Release(ctx)
				}

				// 更新任务状态为timeout
				task.Status = core.TaskStatusTimeout
				// task.TimeStart = &now  // 超时了，开始时间就不用设置了
				task.TimeEnd = &now

				// 更新任务
				_, err = d.taskStore.Update(ctx, task)
				if err != nil {
					logger.Error("更新超时任务失败", zap.Error(err), zap.String("task_id", task.ID.String()))
				}

				logger.Info("任务已超时", zap.String("task_id", task.ID.String()))
			}(task)
		}

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

		pendingTasks, err := d.taskStore.List(ctx, 0, 100, pendingFilter...)
		if err != nil {
			logger.Error("获取待处理任务失败", zap.Error(err))
			time.Sleep(1 * time.Second) // 出错时暂停1秒后重试
			continue
		}

		// 将待处理任务加入全局队列
		for _, task := range pendingTasks {
			select {
			case pendingTasksQueue <- task:
				// 任务成功加入队列
			default:
				// 队列已满，记录日志
				logger.Warn("待处理任务队列已满", zap.String("task_id", task.ID.String()))
			}
		}

		// 每500毫秒检查一次
		time.Sleep(500 * time.Millisecond)
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
