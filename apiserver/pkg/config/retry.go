package config

import (
	"strconv"
	"time"
)

// retry 重试配置（全局运行时参数）
//
// 控制任务重试机制的全局行为，包括：
// - 全局开关：是否启用重试功能
// - 检查间隔：检查失败任务的频率
// - 延迟策略：指数退避算法参数
//
// 注意：具体的重试次数由CronJob级别配置（max_retry, retryable）
type retry struct {
	Enabled       bool          // 全局开关：是否启用重试功能
	CheckInterval time.Duration // 检查失败任务的间隔
	BaseDelay     time.Duration // 重试基础延迟（第1次重试）
	MaxDelay      time.Duration // 重试最大延迟（上限）
	Multiplier    float64       // 延迟倍数（指数退避）
}

var Retry *retry

// parseRetry 解析重试配置
//
// 从环境变量读取配置，支持的环境变量：
// - RETRY_ENABLED: 全局开关（默认：true）
// - RETRY_CHECK_INTERVAL: 检查间隔（秒，默认：60）
// - RETRY_BASE_DELAY: 基础延迟（秒，默认：60）
// - RETRY_MAX_DELAY: 最大延迟（秒，默认：3600）
// - RETRY_MULTIPLIER: 延迟倍数（默认：2.0）
func parseRetry() {
	enabledStr := GetDefaultEnv("RETRY_ENABLED", "true")
	checkIntervalStr := GetDefaultEnv("RETRY_CHECK_INTERVAL", "60") // 秒
	baseDelayStr := GetDefaultEnv("RETRY_BASE_DELAY", "60")         // 秒
	maxDelayStr := GetDefaultEnv("RETRY_MAX_DELAY", "3600")         // 秒
	multiplierStr := GetDefaultEnv("RETRY_MULTIPLIER", "2.0")

	// 解析布尔值
	enabled := enabledStr == "true" || enabledStr == "1"

	// 解析整数
	checkInterval, _ := strconv.Atoi(checkIntervalStr)
	if checkInterval <= 0 {
		checkInterval = 60
	}

	baseDelay, _ := strconv.Atoi(baseDelayStr)
	if baseDelay <= 0 {
		baseDelay = 60
	}

	maxDelay, _ := strconv.Atoi(maxDelayStr)
	if maxDelay <= 0 {
		maxDelay = 3600
	}

	multiplier, err := strconv.ParseFloat(multiplierStr, 64)
	if err != nil || multiplier <= 0 {
		multiplier = 2.0
	}

	Retry = &retry{
		Enabled:       enabled,
		CheckInterval: time.Duration(checkInterval) * time.Second,
		BaseDelay:     time.Duration(baseDelay) * time.Second,
		MaxDelay:      time.Duration(maxDelay) * time.Second,
		Multiplier:    multiplier,
	}
}

func init() {
	parseRetry()
}
