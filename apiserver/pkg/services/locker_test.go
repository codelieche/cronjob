package services

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// TestRedisLocker_AcquireAndRelease 测试获取和释放锁的基本功能
func TestRedisLocker_AcquireAndRelease(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker()
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:acquire_release"
	lockExpire := 10 * time.Second

	// 创建上下文
	ctx := context.Background()

	// 获取锁
	lock, err := locker.Acquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// 检查锁是否有效
	isLocked, err := lock.IsLocked(ctx)
	assert.NoError(t, err)
	assert.True(t, isLocked)

	// 释放锁
	err = lock.Release(ctx)
	assert.NoError(t, err)

	// 再次检查锁是否有效（应该无效）
	isLocked, err = lock.IsLocked(ctx)
	assert.NoError(t, err)
	assert.False(t, isLocked)
}

// TestRedisLocker_TryAcquire 测试尝试获取已被占用的锁
func TestRedisLocker_TryAcquire(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker()
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:try_acquire"
	lockExpire := 10 * time.Second

	// 创建上下文
	ctx := context.Background()

	// 第一个协程获取锁
	lock1, err := locker.Acquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock1)

	// 第二个协程尝试获取同一个锁（应该失败）
	lock2, err := locker.TryAcquire(ctx, lockKey, lockExpire)
	assert.Error(t, err)
	assert.Nil(t, lock2)

	// 释放第一个锁
	err = lock1.Release(ctx)
	assert.NoError(t, err)

	// 再次尝试获取锁（应该成功）
	lock2, err = locker.TryAcquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock2)

	// 释放第二个锁
	err = lock2.Release(ctx)
	assert.NoError(t, err)
}

// TestRedisLocker_Refresh 测试续租锁的功能
func TestRedisLocker_Refresh(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker()
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:refresh"
	initialExpire := 5 * time.Second
	refreshExpire := 10 * time.Second

	// 创建上下文
	ctx := context.Background()

	// 获取锁
	lock, err := locker.Acquire(ctx, lockKey, initialExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// 等待2秒后续租锁
	time.Sleep(2 * time.Second)
	err = lock.Refresh(ctx, refreshExpire)
	assert.NoError(t, err)

	// 再次等待6秒，检查锁是否仍然有效
	// 如果没有成功续租，锁应该已经过期
	time.Sleep(6 * time.Second)
	isLocked, err := lock.IsLocked(ctx)
	assert.NoError(t, err)
	assert.True(t, isLocked) // 锁应该仍然有效，因为我们续租了

	// 释放锁
	err = lock.Release(ctx)
	assert.NoError(t, err)
}

// TestRedisLocker_AutoRefresh 测试自动续租锁的功能
func TestRedisLocker_AutoRefresh(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker()
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:auto_refresh"
	lockExpire := 10 * time.Second
	refreshInterval := 3 * time.Second

	// 创建上下文
	ctx := context.Background()

	// 获取锁
	lock, err := locker.Acquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// 启动自动续租
	stopAutoRefresh, err := lock.AutoRefresh(ctx, lockExpire, refreshInterval)
	assert.NoError(t, err)
	assert.NotNil(t, stopAutoRefresh)

	// 等待一段时间（超过初始过期时间，但在自动续租的保护下）
	time.Sleep(15 * time.Second)

	// 检查锁是否仍然有效
	isLocked, err := lock.IsLocked(ctx)
	assert.NoError(t, err)
	assert.True(t, isLocked)

	// 停止自动续租
	stopAutoRefresh()

	// 释放锁
	err = lock.Release(ctx)
	assert.NoError(t, err)
}

// TestRedisLocker_ConcurrentAccess 测试并发访问同一个锁
func TestRedisLocker_ConcurrentAccess(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker(&RedisLockerOptions{
		RetryCount:    10,
		RetryInterval: 100 * time.Millisecond,
	})
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:concurrent"
	lockExpire := 5 * time.Second

	// 创建等待组，用于等待所有协程完成
	var wg sync.WaitGroup

	// 记录成功获取锁的次数
	var successCount int32 = 0
	log.Printf("开始测试并发访问同一个锁, successCount: %d", successCount)

	// 启动多个协程尝试获取同一个锁
	goroutineCount := 5
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(id int) {
			defer wg.Done()

			// 创建上下文
			ctx := context.Background()

			// 尝试获取锁
			lock, err := locker.Acquire(ctx, lockKey, lockExpire)
			if err != nil {
				t.Logf("协程 %d 获取锁失败: %v", id, err)
				return
			}

			// 成功获取锁
			t.Logf("协程 %d 成功获取锁", id)

			// 模拟执行业务逻辑
			time.Sleep(1 * time.Second)

			// 释放锁
			if err := lock.Release(ctx); err != nil {
				t.Logf("协程 %d 释放锁失败: %v", id, err)
			} else {
				t.Logf("协程 %d 成功释放锁", id)
			}
		}(i)
	}

	// 等待所有协程完成
	wg.Wait()

	// 此时锁应该已经被释放
	ctx := context.Background()
	redisClient, err := core.GetRedis()
	assert.NoError(t, err)

	// 检查锁是否存在
	val, err := redisClient.Get(ctx, lockKey).Result()
	if err != redis.Nil {
		t.Errorf("锁应该已经被释放，但实际状态: %v, %v", val, err)
	}
}

// TestRedisLocker_LockExpiration 测试锁的过期功能
func TestRedisLocker_LockExpiration(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker()
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:expiration"
	lockExpire := 3 * time.Second

	// 创建上下文
	ctx := context.Background()

	// 获取锁
	lock, err := locker.Acquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// 等待锁过期
	time.Sleep(4 * time.Second)

	// 检查锁是否仍然有效（应该无效）
	isLocked, err := lock.IsLocked(ctx)
	assert.NoError(t, err)
	assert.False(t, isLocked)

	// 尝试释放已过期的锁（应该失败）
	err = lock.Release(ctx)
	assert.Error(t, err)

	// 尝试再次获取同一个锁（应该成功，因为锁已经过期）
	newLock, err := locker.Acquire(ctx, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, newLock)

	// 释放新获取的锁
	err = newLock.Release(ctx)
	assert.NoError(t, err)
}

// TestRedisLocker_ContextCancellation 测试上下文取消对获取锁的影响
func TestRedisLocker_ContextCancellation(t *testing.T) {
	// 创建锁管理器
	locker, err := NewRedisLocker(&RedisLockerOptions{
		RetryCount:    20, // 增加重试次数，确保上下文取消能被检测到
		RetryInterval: 100 * time.Millisecond,
	})
	assert.NoError(t, err)
	assert.NotNil(t, locker)

	// 定义锁的键名和过期时间
	lockKey := "test:lock:context_cancellation"
	lockExpire := 10 * time.Second

	// 第一个协程先获取锁
	ctx1 := context.Background()
	lock1, err := locker.Acquire(ctx1, lockKey, lockExpire)
	assert.NoError(t, err)
	assert.NotNil(t, lock1)

	// 第二个协程使用带超时的上下文尝试获取同一个锁
	ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	startTime := time.Now()
	lock2, err := locker.Acquire(ctx2, lockKey, lockExpire)
	duration := time.Since(startTime)

	// 应该因为上下文超时而失败
	assert.Error(t, err)
	assert.Nil(t, lock2)
	assert.True(t, duration >= 2*time.Second, "获取锁的时间应该大于等于上下文超时时间")

	// 释放第一个锁
	err = lock1.Release(ctx1)
	assert.NoError(t, err)
}
