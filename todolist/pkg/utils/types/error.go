package types

import "errors"

// ErrNotFound 未找到
var ErrNotFound = errors.New("Not Found")

// ErrUnauthorized token 校验错误
var ErrUnauthorized = errors.New("unauthorized")
