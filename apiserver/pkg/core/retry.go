package core

import (
	"math"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
)

// CalculateNextRetryTime 计算下次重试时间（使用配置）
//
// 使用指数退避算法计算下次重试时间：
// - 第1次重试：baseDelay = 1分钟
// - 第2次重试：baseDelay * multiplier^1 = 2分钟
// - 第3次重试：baseDelay * multiplier^2 = 4分钟
// - 第4次重试：baseDelay * multiplier^3 = 8分钟
// - ...
// - 最大延迟：maxDelay = 60分钟
//
// 参数:
//   - retryCount: 当前重试次数（0表示第一次失败，1表示第一次重试失败）
//   - failureTime: 失败时间（通常是当前时间）
//
// 返回:
//   - time.Time: 下次重试的时间点
//
// 示例:
//
//	nextRetryTime := CalculateNextRetryTime(0, time.Now()) // 第1次重试，1分钟后
//	nextRetryTime := CalculateNextRetryTime(1, time.Now()) // 第2次重试，2分钟后
func CalculateNextRetryTime(retryCount int, failureTime time.Time) time.Time {
	// 指数退避：delay = baseDelay * (multiplier ^ retryCount)
	delay := float64(config.Retry.BaseDelay) * math.Pow(config.Retry.Multiplier, float64(retryCount))

	// 限制最大延迟
	if delay > float64(config.Retry.MaxDelay) {
		delay = float64(config.Retry.MaxDelay)
	}

	return failureTime.Add(time.Duration(delay))
}

// ShouldRetry 判断任务是否应该重试
//
// 判断逻辑：
// 1. 任务必须标记为可重试（retryable = true）
// 2. 重试次数未达到最大限制（retry_count < max_retry）
// 3. 任务状态为失败状态（failed/error/timeout）
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

	// 3. 检查任务状态
	failedStatuses := map[string]bool{
		TaskStatusFailed:  true,
		TaskStatusError:   true,
		TaskStatusTimeout: true,
	}

	return failedStatuses[task.Status]
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
