package core

import (
	"context"

	"github.com/codelieche/todolist/pkg/config"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// redisClient 全局Redis客户端实例
var redisClient *redis.Client

// init 初始化函数，在包加载时自动连接Redis
func init() {
	// 初始化Redis连接
	var err error
	if redisClient, err = connectRedis(); err != nil {
		logger.Warn("Redis初始化连接失败，将在首次使用时重新尝试", zap.Error(err))
	}
}

// GetRedis 获取Redis客户端实例
// 如果连接不存在，会尝试重新创建连接
// 返回: *redis.Client - Redis客户端实例, error - 错误信息
func GetRedis() (*redis.Client, error) {
	// 检查连接是否已存在且可用
	if redisClient != nil {
		// 简单测试连接是否可用
		ctx := context.Background()
		_, err := redisClient.Ping(ctx).Result()
		if err == nil {
			return redisClient, nil
		}
		logger.Warn("Redis连接不可用，尝试重新连接", zap.Error(err))
	}

	// 连接不存在或不可用时，创建新连接
	var err error
	redisClient, err = connectRedis()
	return redisClient, err
}

// connectRedis 内部函数：创建Redis连接并配置连接池
// 返回: *redis.Client - Redis客户端实例, error - 错误信息
func connectRedis() (*redis.Client, error) {
	// 获取Redis配置
	redisConfig := config.Redis

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     redisConfig.GetAddr(), // Redis服务器地址
		Password: redisConfig.Password,  // Redis密码
		DB:       redisConfig.DB,        // Redis数据库编号
		PoolSize: redisConfig.PoolSize,  // 连接池大小
	})

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	logger.Info("Redis连接成功", zap.String("address", redisConfig.GetAddr()))
	return client, nil
}

// CloseRedis 关闭Redis连接
// 在应用程序关闭时调用此函数释放资源
func CloseRedis() error {
	if redisClient != nil {
		logger.Info("关闭Redis连接")
		return redisClient.Close()
	}
	return nil
}
