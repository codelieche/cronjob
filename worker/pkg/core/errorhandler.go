package core

import (
	"context"

	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"go.uber.org/zap"
)

// ErrorLevel 错误级别
type ErrorLevel int

const (
	LevelInfo ErrorLevel = iota
	LevelWarn
	LevelError
	LevelFatal
)

// String 返回错误级别的字符串表示
func (s ErrorLevel) String() string {
	switch s {
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ErrorContext 错误上下文信息
type ErrorContext struct {
	TaskID    string                 `json:"task_id,omitempty"` // 任务ID
	Component string                 `json:"component"`         // 组件名称
	Action    string                 `json:"action"`            // 执行的动作
	Level     ErrorLevel             `json:"level"`             // 错误级别
	Extra     map[string]interface{} `json:"extra,omitempty"`   // 额外信息
}

// ErrorHandler 统一错误处理接口
//
// 提供统一的错误处理机制，包括：
// 1. 统一的日志记录格式
// 2. 错误分级处理
// 3. 任务错误的状态更新
// 4. 系统错误的监控上报
type ErrorHandler interface {
	// HandleTaskError 处理任务执行错误
	//
	// 参数:
	//   - ctx: 上下文信息
	//   - err: 错误对象
	//   - context: 错误上下文信息
	//
	// 功能:
	//   - 记录错误日志
	//   - 发送任务状态更新
	//   - 根据级别决定是否上报
	HandleTaskError(ctx context.Context, err error, context ErrorContext)

	// HandleSystemError 处理系统级错误
	//
	// 参数:
	//   - ctx: 上下文信息
	//   - err: 错误对象
	//   - context: 错误上下文信息
	//
	// 功能:
	//   - 记录错误日志
	//   - 系统错误监控
	//   - 必要时触发告警
	HandleSystemError(ctx context.Context, err error, context ErrorContext)

	// HandleRecoverableError 处理可恢复错误
	//
	// 参数:
	//   - ctx: 上下文信息
	//   - err: 错误对象
	//   - context: 错误上下文信息
	//
	// 功能:
	//   - 记录警告日志
	//   - 尝试自动恢复
	//   - 统计错误频率
	HandleRecoverableError(ctx context.Context, err error, context ErrorContext)
}

// errorHandlerImpl 错误处理器实现
type errorHandlerImpl struct {
	updateCallback TaskUpdateCallback // 任务更新回调
}

// NewErrorHandler 创建错误处理器实例
//
// 参数:
//   - updateCallback: 任务更新回调，用于发送任务状态更新
//
// 返回值:
//   - ErrorHandler: 错误处理器接口
func NewErrorHandler(updateCallback TaskUpdateCallback) ErrorHandler {
	return &errorHandlerImpl{
		updateCallback: updateCallback,
	}
}

// HandleTaskError 处理任务执行错误
func (e *errorHandlerImpl) HandleTaskError(ctx context.Context, err error, context ErrorContext) {
	// 构建日志字段
	fields := []zap.Field{
		zap.Error(err),
		zap.String("component", context.Component),
		zap.String("action", context.Action),
		zap.String("level", context.Level.String()),
	}

	if context.TaskID != "" {
		fields = append(fields, zap.String("task_id", context.TaskID))
	}

	// 添加额外字段
	for key, value := range context.Extra {
		fields = append(fields, zap.Any(key, value))
	}

	// 根据级别记录日志
	switch context.Level {
	case LevelInfo:
		logger.Info("任务执行信息", fields...)
	case LevelWarn:
		logger.Warn("任务执行警告", fields...)
	case LevelError:
		logger.Error("任务执行错误", fields...)
	case LevelFatal:
		logger.Error("任务执行致命错误", fields...)
	}

	// 发送任务状态更新
	if context.TaskID != "" && e.updateCallback != nil {
		e.sendTaskErrorUpdate(context.TaskID, err, context)
	}
}

// HandleSystemError 处理系统级错误
func (e *errorHandlerImpl) HandleSystemError(ctx context.Context, err error, context ErrorContext) {
	// 构建日志字段
	fields := []zap.Field{
		zap.Error(err),
		zap.String("component", context.Component),
		zap.String("action", context.Action),
		zap.String("level", context.Level.String()),
	}

	// 添加额外字段
	for key, value := range context.Extra {
		fields = append(fields, zap.Any(key, value))
	}

	// 记录系统错误日志
	switch context.Level {
	case LevelInfo:
		logger.Info("系统信息", fields...)
	case LevelWarn:
		logger.Warn("系统警告", fields...)
	case LevelError:
		logger.Error("系统错误", fields...)
	case LevelFatal:
		logger.Error("系统致命错误", fields...)
		// 致命错误可能需要特殊处理，比如触发告警
	}
}

// HandleRecoverableError 处理可恢复错误
func (e *errorHandlerImpl) HandleRecoverableError(ctx context.Context, err error, context ErrorContext) {
	// 构建日志字段
	fields := []zap.Field{
		zap.Error(err),
		zap.String("component", context.Component),
		zap.String("action", context.Action),
		zap.String("level", context.Level.String()),
	}

	// 添加额外字段
	for key, value := range context.Extra {
		fields = append(fields, zap.Any(key, value))
	}

	// 记录可恢复错误日志
	logger.Warn("可恢复错误", fields...)

	// 这里可以添加自动恢复逻辑
	// 比如重连、重试等
}

// sendTaskErrorUpdate 发送任务错误状态更新
func (e *errorHandlerImpl) sendTaskErrorUpdate(taskID string, err error, context ErrorContext) {
	// 构建错误结果数据
	errorData := map[string]interface{}{
		"status": "error",
		"error":  err.Error(),
	}

	// 根据错误类型设置不同的状态
	switch context.Level {
	case LevelFatal:
		errorData["status"] = "failed"
	case LevelError:
		errorData["status"] = "error"
	case LevelWarn:
		// 警告级别不改变任务状态，只记录日志
		return
	case LevelInfo:
		// 信息级别不改变任务状态
		return
	}

	// 添加组件和动作信息
	errorData["component"] = context.Component
	errorData["action"] = context.Action

	// 发送状态更新
	if updateErr := e.updateCallback.SendTaskUpdate(taskID, errorData); updateErr != nil {
		logger.Error("发送任务错误状态更新失败",
			zap.String("task_id", taskID),
			zap.Error(updateErr),
			zap.Error(err)) // 原始错误
	}
}

// 注意：任务状态常量已在其他文件中定义，这里不需要重复定义
