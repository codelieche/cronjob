package config

import (
	"strconv"
	"time"
)

// retry 重试配置（全局运行时参数）
//
// 控制任务重试机制的全局行为：
// - 全局开关：是否启用重试功能
// - 检查间隔：检查失败任务的频率
//
// 🔥 重试策略：立即重试，不使用延迟
// - 任务失败时，next_retry_time 设置为 NOW
// - checkFailedTasks 立即检测并创建重试任务
//
// 注意：具体的重试次数由CronJob级别配置（max_retry, retryable）
type retry struct {
	Enabled       bool          // 全局开关：是否启用重试功能
	CheckInterval time.Duration // 检查失败任务的间隔（默认30秒）
}

var Retry *retry

// parseRetry 解析重试配置
//
// 从环境变量读取配置，支持的环境变量：
// - RETRY_ENABLED: 全局开关（默认：true）
// - RETRY_CHECK_INTERVAL: 检查间隔（秒，默认：30）
func parseRetry() {
	enabledStr := GetDefaultEnv("RETRY_ENABLED", "true")
	checkIntervalStr := GetDefaultEnv("RETRY_CHECK_INTERVAL", "30") // 秒

	// 解析布尔值
	enabled := enabledStr == "true" || enabledStr == "1"

	// 解析检查间隔
	checkInterval, _ := strconv.Atoi(checkIntervalStr)
	if checkInterval <= 0 {
		checkInterval = 30 // 默认30秒
	}

	Retry = &retry{
		Enabled:       enabled,
		CheckInterval: time.Duration(checkInterval) * time.Second,
	}
}

func init() {
	parseRetry()
}
