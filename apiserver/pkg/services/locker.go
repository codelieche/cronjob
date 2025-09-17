package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisLocker 是基于Redis实现的分布式锁管理器
type RedisLocker struct {
	client *redis.Client
	opts   *RedisLockerOptions
}

// RedisLockerOptions 包含Redis锁管理器的配置选项

type RedisLockerOptions struct {
	// RetryCount 尝试获取锁的重试次数
	RetryCount int
	// RetryInterval 尝试获取锁的重试间隔
	RetryInterval time.Duration
}

// NewRedisLocker 创建一个新的Redis锁管理器
// 如果不传入options，则使用默认配置
func NewRedisLocker(options ...*RedisLockerOptions) (*RedisLocker, error) {
	// 获取Redis客户端
	client, err := core.GetRedis()
	if err != nil {
		return nil, err
	}

	// 设置默认选项
	opts := &RedisLockerOptions{
		RetryCount:    3,
		RetryInterval: 100 * time.Millisecond,
	}

	// 如果传入了选项，则覆盖默认选项
	if len(options) > 0 && options[0] != nil {
		if options[0].RetryCount > 0 {
			opts.RetryCount = options[0].RetryCount
		}
		if options[0].RetryInterval > 0 {
			opts.RetryInterval = options[0].RetryInterval
		}
	}

	return &RedisLocker{
		client: client,
		opts:   opts,
	}, nil
}

// Acquire 实现Locker接口的Acquire方法
func (rl *RedisLocker) Acquire(ctx context.Context, key string, expire time.Duration) (core.Lock, error) {
	// 生成随机值作为锁的值，用于验证锁的拥有者
	val, err := generateRandomValue()
	if err != nil {
		return nil, err
	}

	// 创建锁实例
	lock := &redisLock{
		client: rl.client,
		key:    key,
		value:  val,
	}

	// 尝试获取锁
	success, err := lock.acquire(ctx, expire)
	if err != nil {
		return nil, err
	}

	// 如果获取成功，返回锁实例
	if success {
		return lock, nil
	}

	// 如果获取失败，重试
	retryCount := 0
	for retryCount < rl.opts.RetryCount {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(rl.opts.RetryInterval):
			retryCount++
			success, err = lock.acquire(ctx, expire)
			if err != nil {
				return nil, err
			}
			if success {
				return lock, nil
			}
		}
	}

	return nil, errors.New("failed to acquire lock after retries")
}

// TryAcquire 实现Locker接口的TryAcquire方法
func (rl *RedisLocker) TryAcquire(ctx context.Context, key string, expire time.Duration) (core.Lock, error) {
	// 生成随机值作为锁的值
	val, err := generateRandomValue()
	if err != nil {
		return nil, err
	}

	// 创建锁实例
	lock := &redisLock{
		client: rl.client,
		key:    key,
		value:  val,
	}

	// 尝试获取锁
	success, err := lock.acquire(ctx, expire)
	if err != nil {
		return nil, err
	}

	// 如果获取成功，返回锁实例；否则返回错误
	if success {
		return lock, nil
	}

	return nil, core.ErrLockAlreadyAcquired
}

// ReleaseByKeyAndValue 实现Locker接口的ReleaseByKeyAndValue方法
// 允许在不同HTTP请求之间通过key和value直接释放锁
func (rl *RedisLocker) ReleaseByKeyAndValue(ctx context.Context, key, value string) error {
	// 使用Lua脚本确保原子性地释放锁
	// 只有锁的拥有者才能释放锁，防止误释放
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`

	// 替换Lua脚本中的参数
	script = replaceLuaScriptParams(script, "ARGV[1]", value)

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{key}).Result()
	if err != nil {
		logger.Error("failed to release lock by key and value", zap.String("key", key), zap.Error(err))
		return err
	}

	// 检查结果
	if i, ok := result.(int64); !ok || i == 0 {
		return errors.New("failed to release lock: lock not owned by this instance or already released")
	}

	return nil
}

// CheckLock 实现Locker接口的CheckLock方法
// 检查指定键名的锁是否存在且值匹配
func (rl *RedisLocker) CheckLock(ctx context.Context, key string, value string) (bool, string, error) {
	// 从Redis获取锁的值
	storedValue, err := rl.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// 锁不存在
			return false, "", nil
		}
		logger.Error("failed to check lock status", zap.String("key", key), zap.Error(err))
		return false, "", err
	}

	// 锁存在，检查值是否匹配（如果提供了value参数）
	if value != "" && storedValue != value {
		return false, storedValue, nil
	}

	// 锁存在且值匹配（或未提供value参数）
	return true, storedValue, nil
}

// RefreshLock 实现Locker接口的RefreshLock方法
// 续租指定键名的锁
func (rl *RedisLocker) RefreshLock(ctx context.Context, key string, value string, expire time.Duration) error {
	// 使用Lua脚本确保原子性地续租锁
	// 只有锁的拥有者才能续租锁
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
	`

	// 替换脚本参数
	expireMs := int64(expire / time.Millisecond)
	script = replaceLuaScriptParams(script, "ARGV[1]", value)
	script = replaceLuaScriptParams(script, "ARGV[2]", expireMs)

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{key}).Result()
	if err != nil {
		logger.Error("failed to refresh lock", zap.String("key", key), zap.Error(err))
		return err
	}

	// 检查结果
	if i, ok := result.(int64); !ok || i == 0 {
		return errors.New("failed to refresh lock: lock not owned by this instance or already expired")
	}

	return nil
}

// redisLock 是基于Redis实现的单个锁

type redisLock struct {
	client *redis.Client
	key    string
	value  string
}

// acquire 尝试获取锁
func (rl *redisLock) acquire(ctx context.Context, expire time.Duration) (bool, error) {
	// 使用SET命令的NX选项实现互斥锁
	// NX: 只有当键不存在时才设置值
	// PX: 设置键的过期时间（毫秒）
	success, err := rl.client.SetNX(ctx, rl.key, rl.value, expire).Result()
	if err != nil {
		logger.Error("failed to acquire lock", zap.String("key", rl.key), zap.Error(err))
		return false, err
	}

	return success, nil
}

// Release 实现Lock接口的Release方法
func (rl *redisLock) Release(ctx context.Context) error {
	// 使用Lua脚本确保原子性地释放锁
	// 只有锁的拥有者才能释放锁，防止误释放
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`

	// 替换Lua脚本中的参数
	script = replaceLuaScriptParams(script, "ARGV[1]", rl.value)

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{rl.key}).Result()
	if err != nil {
		logger.Error("failed to release lock", zap.String("key", rl.key), zap.Error(err))
		return err
	}

	// 检查结果
	if i, ok := result.(int64); !ok || i == 0 {
		return errors.New("failed to release lock: lock not owned by this instance or already released")
	}

	return nil
}

// Refresh 实现Lock接口的Refresh方法
func (rl *redisLock) Refresh(ctx context.Context, expire time.Duration) error {
	// 使用Lua脚本确保原子性地续租锁
	// 只有锁的拥有者才能续租锁
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
	`

	// 替换Lua脚本中的参数
	expireMs := int64(expire / time.Millisecond)
	script = replaceLuaScriptParams(script, "ARGV[1]", rl.value)
	script = replaceLuaScriptParams(script, "ARGV[2]", expireMs)

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{rl.key}).Result()
	if err != nil {
		logger.Error("failed to refresh lock", zap.String("key", rl.key), zap.Error(err))
		return err
	}

	// 检查结果
	if i, ok := result.(int64); !ok || i == 0 {
		return errors.New("failed to refresh lock: lock not owned by this instance or already expired")
	}

	return nil
}

// AutoRefresh 实现Lock接口的AutoRefresh方法
func (rl *redisLock) AutoRefresh(ctx context.Context, expire time.Duration, interval time.Duration) (func(), error) {
	// 创建一个新的上下文，用于控制自动续租的生命周期
	ctx, cancel := context.WithCancel(ctx)

	// 启动自动续租goroutine
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Debug("stopping auto-refresh for lock", zap.String("key", rl.key))
				return
			case <-ticker.C:
				// 执行续租操作
				if err := rl.Refresh(ctx, expire); err != nil {
					logger.Error("failed to auto-refresh lock", zap.String("key", rl.key), zap.Error(err))
					// 如果续租失败，可以选择继续尝试或者停止
				}
			}
		}
	}()

	// 返回停止自动续租的函数
	return cancel, nil
}

// Key 实现Lock接口的Key方法
func (rl *redisLock) Key() string {
	return rl.key
}

// Value 实现Lock接口的Value方法
// 返回锁的值，用于在HTTP API场景下传递锁的信息
func (rl *redisLock) Value() string {
	return rl.value
}

// IsLocked 实现Lock接口的IsLocked方法
func (rl *redisLock) IsLocked(ctx context.Context) (bool, error) {
	// 检查锁是否存在并且值匹配
	val, err := rl.client.Get(ctx, rl.key).Result()
	if err != nil {
		if err == redis.Nil {
			// 锁不存在
			return false, nil
		}
		logger.Error("failed to check lock status", zap.String("key", rl.key), zap.Error(err))
		return false, err
	}

	// 锁存在，检查值是否匹配
	return val == rl.value, nil
}

// generateRandomValue 生成随机值作为锁的值
func generateRandomValue() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// replaceLuaScriptParams 替换Lua脚本中的参数
func replaceLuaScriptParams(script string, param string, value interface{}) string {
	// 简单的字符串替换实现
	// 在实际应用中，可以使用更复杂的替换逻辑
	// 这里为了简洁，直接使用字符串替换
	// 注意：这个实现有局限性，只适用于简单的参数替换
	// 对于复杂的Lua脚本，可能需要更复杂的解析和替换逻辑

	// 将参数转换为字符串
	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = "'" + v + "'"
	case int:
		valueStr = strconv.Itoa(v)
	case int64:
		valueStr = strconv.FormatInt(v, 10)
	case float64:
		valueStr = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		// 对于其他类型，尝试使用fmt.Sprintf转换
		valueStr = fmt.Sprintf("%v", v)
	}

	// 替换参数
	return replaceString(script, param, valueStr)
}

// replaceString 简单的字符串替换函数
func replaceString(s, old, new string) string {
	result := ""
	current := 0
	for {
		index := findString(s, old, current)
		if index == -1 {
			result += s[current:]
			break
		}
		result += s[current:index] + new
		current = index + len(old)
	}
	return result
}

// findString 简单的字符串查找函数
func findString(s, substr string, start int) int {
	if len(s) < start || start < 0 {
		return -1
	}
	s = s[start:]
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i + start
		}
	}
	return -1
}
