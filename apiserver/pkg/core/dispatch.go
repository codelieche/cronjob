package core

import "context"

type DispatchService interface {
	// Dispatch 调度cronjob
	Dispatch(ctx context.Context, cronJob *CronJob) error
	// DispatchLoop 循环调度CronJob，生产任务清单：使用goroutine运行
	DispatchLoop(ctx context.Context) error
	// CheckTaskLoop 检查任务是否过期：使用goroutine运行
	CheckTaskLoop(ctx context.Context) error
	// Stop 停止任务
	Stop(ctx context.Context, task *Task) error
	// GetTasks 获取任务列表
	GetPendingTasks(ctx context.Context) ([]*Task, error)

	// 🔥 任务自动重试功能
	// CheckFailedTasksLoop 检查失败任务并自动重试：使用goroutine运行
	CheckFailedTasksLoop(ctx context.Context) error
	// RetryTask 手动重试任务
	RetryTask(ctx context.Context, taskID string) (*Task, error)
}
