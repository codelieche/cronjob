package config

import (
	"strconv"
)

// redis Redis配置结构体
type redis struct {
	Host     string // Redis服务器地址
	Port     int    // Redis服务器端口
	Password string // Redis密码
	DB       int    // Redis数据库编号
	PoolSize int    // 连接池大小
}

// GetAddr 获取Redis连接地址
func (r *redis) GetAddr() string {
	return r.Host + ":" + strconv.Itoa(r.Port)
}

// Redis 全局Redis配置实例
var Redis *redis

// parseRedis 解析Redis配置
func parseRedis() {
	host := GetDefaultEnv("REDIS_HOST", "127.0.0.1")
	portStr := GetDefaultEnv("REDIS_PORT", "6379")
	password := GetDefaultEnv("REDIS_PASSWORD", "")
	dbStr := GetDefaultEnv("REDIS_DB", "0")
	poolSizeStr := GetDefaultEnv("REDIS_POOL_SIZE", "10")

	// 解析端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 6379
	}

	// 解析数据库编号
	db, err := strconv.Atoi(dbStr)
	if err != nil {
		db = 0
	}

	// 解析连接池大小
	poolSize, err := strconv.Atoi(poolSizeStr)
	if err != nil {
		poolSize = 10
	}

	// 设置Redis配置
	Redis = &redis{
		host,
		port,
		password,
		db,
		poolSize,
	}
}

// 初始化Redis配置
func init() {
	parseRedis()
}