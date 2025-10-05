package core

import (
	"time"
)

// ShouldRetry 判断任务是否应该重试
//
// 判断逻辑：
// 1. 任务必须标记为可重试（retryable = true）
// 2. 重试次数未达到最大限制（retry_count < max_retry）
// 3. 任务状态为失败状态（failed/error，不包括 timeout）
// 4. 任务未超过 TimeoutAt 宽限期
//
// 注意：timeout 任务不重试，因为：
//   - timeout 说明任务执行时间太长
//   - 新的调度周期会产生新任务
//   - 上一个周期的任务已经不重要了
//
// 参数:
//   - task: 任务对象
//
// 返回:
//   - bool: true表示应该重试，false表示不应该重试
func ShouldRetry(task *Task) bool {
	// 1. 检查是否可重试
	if task.Retryable == nil || !*task.Retryable {
		return false
	}

	// 2. 检查重试次数
	if task.RetryCount >= task.MaxRetry {
		return false
	}

	// 3. 检查任务状态（只重试 failed 和 error，不重试 timeout）
	failedStatuses := map[string]bool{
		TaskStatusFailed: true,
		TaskStatusError:  true,
		// 🔥 不包括 TaskStatusTimeout（新调度周期会产生新任务）
	}

	if !failedStatuses[task.Status] {
		return false
	}

	// 🔥 4. 检查任务是否已经超时太久（防止无意义的重试）
	// 如果任务的超时时间点已经过去太久（超过30分钟），就不再重试
	if !task.TimeoutAt.IsZero() {
		now := time.Now()
		// 给予30分钟的宽限期（可以根据实际情况调整）
		maxGracePeriod := 30 * time.Minute
		if now.Sub(task.TimeoutAt) > maxGracePeriod {
			return false
		}
	}

	return true
}

// IsRetryReady 判断任务是否已到重试时间
//
// 判断逻辑：
// 1. 任务应该重试（ShouldRetry返回true）
// 2. 已设置下次重试时间
// 3. 当前时间已达到或超过下次重试时间
//
// 参数:
//   - task: 任务对象
//   - now: 当前时间
//
// 返回:
//   - bool: true表示可以立即重试，false表示还需等待
func IsRetryReady(task *Task, now time.Time) bool {
	// 1. 检查是否应该重试
	if !ShouldRetry(task) {
		return false
	}

	// 2. 检查是否设置了下次重试时间
	if task.NextRetryTime == nil {
		return false
	}

	// 3. 检查是否已到重试时间
	return now.After(*task.NextRetryTime) || now.Equal(*task.NextRetryTime)
}
