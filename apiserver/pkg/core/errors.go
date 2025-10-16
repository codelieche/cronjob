package core

import (
	"errors"
)

var (
	// ErrNotFound 记录不存在
	ErrNotFound = errors.New("记录不存在")
	// ErrConflict 记录冲突（已存在）
	ErrConflict = errors.New("记录已存在")
	// ErrBadRequest 请求参数错误
	ErrBadRequest = errors.New("请求参数错误")
	// ErrNotImplemented 功能未实现
	ErrNotImplemented = errors.New("功能未实现")

	// ErrUnauthorized token 校验错误
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden 权限不足
	ErrForbidden = errors.New("权限不足")
	// ErrInternalServerError 内部服务器错误
	ErrInternalServerError = errors.New("内部服务器错误")
	// ErrServiceUnavailable 服务不可用
	ErrServiceUnavailable = errors.New("服务不可用")

	// ErrLockAlreadyAcquired
	ErrLockAlreadyAcquired = errors.New("lock already acquired")
)

// ErrorResponse API错误响应结构体，用于Swagger文档
type ErrorResponse struct {
	Error   string `json:"error" example:"请求参数错误"`   // 错误信息
	Message string `json:"message" example:"参数验证失败"` // 详细错误消息
	Code    int    `json:"code" example:"400"`       // HTTP状态码
}
