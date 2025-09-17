package core

import (
	"context"
	"time"
)

// Locker 接口定义了分布式锁的基本操作
// 这个接口设计使得未来可以轻松切换到etcd或其他存储实现

type Locker interface {
	// Acquire 获取锁
	// 参数:
	//  - ctx: 上下文，用于取消操作
	//  - key: 锁的键名
	//  - expire: 锁的过期时间
	// 返回值:
	//  - lock: 锁的实例，如果获取失败则为nil
	//  - err: 错误信息
	Acquire(ctx context.Context, key string, expire time.Duration) (lock Lock, err error)

	// TryAcquire 尝试获取锁，但如果锁被占用则立即返回失败
	// 参数与Acquire相同
	TryAcquire(ctx context.Context, key string, expire time.Duration) (lock Lock, err error)

	// ReleaseByKeyAndValue 通过key和value直接释放锁
	// 适用于HTTP API场景，不同请求之间无法共享Lock实例的情况
	// 参数:
	//  - ctx: 上下文，用于取消操作
	//  - key: 锁的键名
	//  - value: 锁的值，用于验证锁的拥有者
	// 返回值:
	//  - err: 错误信息
	ReleaseByKeyAndValue(ctx context.Context, key, value string) error

	// CheckLock 检查指定键名的锁是否存在且值匹配
	// 参数:
	//  - ctx: 上下文，用于取消操作
	//  - key: 锁的键名
	//  - value: 锁的值，如果为空则只检查锁是否存在
	// 返回值:
	//  - bool: 锁是否存在且值匹配
	//  - storedValue: 存储在Redis中的锁值
	//  - err: 错误信息
	CheckLock(ctx context.Context, key string, value string) (exists bool, storedValue string, err error)

	// RefreshLock 续租指定键名的锁
	// 参数:
	//  - ctx: 上下文，用于取消操作
	//  - key: 锁的键名
	//  - value: 锁的值，用于验证锁的拥有者
	//  - expire: 新的过期时间
	// 返回值:
	//  - err: 错误信息
	RefreshLock(ctx context.Context, key string, value string, expire time.Duration) error
}

// Lock 接口定义了单个锁的操作

type Lock interface {
	// Release 释放锁
	// 参数:
	//  - ctx: 上下文，用于取消操作
	// 返回值:
	//  - err: 错误信息
	Release(ctx context.Context) error

	// Refresh 续租锁，延长锁的过期时间
	// 参数:
	//  - ctx: 上下文，用于取消操作
	//  - expire: 新的过期时间
	// 返回值:
	//  - err: 错误信息
	Refresh(ctx context.Context, expire time.Duration) error

	// AutoRefresh 自动续租锁，定期刷新锁的过期时间
	// 参数:
	//  - ctx: 上下文，用于取消自动续租
	//  - expire: 锁的过期时间
	//  - interval: 自动续租的间隔时间
	// 返回值:
	//  - stop: 停止自动续租的函数
	//  - err: 错误信息
	AutoRefresh(ctx context.Context, expire time.Duration, interval time.Duration) (stop func(), err error)

	// Key 获取锁的键名
	Key() string

	// Value 获取锁的值
	// 用于在HTTP API场景下，将锁的信息传递给后续请求
	Value() string

	// IsLocked 检查锁是否仍然有效
	// 参数:
	//  - ctx: 上下文，用于取消操作
	// 返回值:
	//  - bool: 锁是否有效
	//  - err: 错误信息
	IsLocked(ctx context.Context) (bool, error)
}
