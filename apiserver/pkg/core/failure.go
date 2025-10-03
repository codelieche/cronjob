package core

// FailureReason 失败原因类型
//
// 定义任务失败的常见原因分类，用于：
// - 判断任务是否可重试
// - 统计分析失败模式
// - 优化系统稳定性
const (
	// 🟢 可重试的失败原因（临时性错误）
	FailureReasonTimeout         = "timeout"          // 任务执行超时（可能是网络慢、任务负载重）
	FailureReasonWorkerError     = "worker_error"     // Worker节点错误（Worker崩溃、重启等）
	FailureReasonNetworkError    = "network_error"    // 网络错误（连接失败、网络抖动）
	FailureReasonResourceError   = "resource_error"   // 资源不足（内存不足、磁盘满等）
	FailureReasonDependencyError = "dependency_error" // 依赖服务暂时不可用

	// 🔴 不可重试的失败原因（永久性错误）
	FailureReasonBadCommand    = "bad_command"    // 命令错误（命令不存在、语法错误）
	FailureReasonPermission    = "permission"     // 权限错误（文件权限、执行权限不足）
	FailureReasonInvalidArgs   = "invalid_args"   // 参数错误（参数格式错误、必需参数缺失）
	FailureReasonConfigError   = "config_error"   // 配置错误（配置文件错误、环境变量缺失）
	FailureReasonBusinessLogic = "business_logic" // 业务逻辑错误（数据不符合预期）

	// ⚪ 未分类的失败原因
	FailureReasonUnknown = "unknown" // 未知错误（无法分类）
)

// IsRetryable 判断失败原因是否可重试
//
// 可重试的失败原因通常是临时性问题，可能在短时间内恢复
// 不可重试的失败原因通常是配置、权限、代码逻辑等问题，需要人工介入
//
// 参数:
//   - reason: 失败原因字符串
//
// 返回:
//   - bool: true表示可重试，false表示不可重试
func IsRetryable(reason string) bool {
	retryableReasons := map[string]bool{
		FailureReasonTimeout:         true,
		FailureReasonWorkerError:     true,
		FailureReasonNetworkError:    true,
		FailureReasonResourceError:   true,
		FailureReasonDependencyError: true,
	}

	return retryableReasons[reason]
}

// ClassifyError 根据错误信息分类失败原因
//
// 通过分析错误消息，自动判断失败原因类型
// 用于Worker端在任务失败时自动设置failure_reason
//
// 参数:
//   - err: 错误对象
//
// 返回:
//   - string: 失败原因分类
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// 超时错误
	if contains(errMsg, "timeout", "timed out", "deadline exceeded") {
		return FailureReasonTimeout
	}

	// 网络错误
	if contains(errMsg, "connection refused", "connection reset", "network", "dial", "EOF") {
		return FailureReasonNetworkError
	}

	// 权限错误
	if contains(errMsg, "permission denied", "access denied", "forbidden", "unauthorized") {
		return FailureReasonPermission
	}

	// 命令错误
	if contains(errMsg, "command not found", "executable file not found", "no such file") {
		return FailureReasonBadCommand
	}

	// 资源错误
	if contains(errMsg, "out of memory", "cannot allocate memory", "disk full", "no space left") {
		return FailureReasonResourceError
	}

	// 参数错误
	if contains(errMsg, "invalid argument", "invalid input", "invalid parameter") {
		return FailureReasonInvalidArgs
	}

	// 依赖服务错误
	if contains(errMsg, "connection refused", "service unavailable", "bad gateway") {
		return FailureReasonDependencyError
	}

	// 未知错误
	return FailureReasonUnknown
}

// contains 检查错误消息是否包含关键词（不区分大小写）
func contains(errMsg string, keywords ...string) bool {
	errMsgLower := toLowerCase(errMsg)
	for _, keyword := range keywords {
		if containsSubstring(errMsgLower, toLowerCase(keyword)) {
			return true
		}
	}
	return false
}

// toLowerCase 转换为小写
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

// containsSubstring 检查字符串是否包含子串
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
