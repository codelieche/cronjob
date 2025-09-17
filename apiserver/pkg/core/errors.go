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
