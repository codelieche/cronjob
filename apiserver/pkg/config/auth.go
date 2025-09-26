package config

import (
	"time"
)

// AuthConfig 认证配置结构体
// 包含认证服务的所有配置参数，支持多种认证方式和性能优化
type AuthConfig struct {
	// 基础配置
	ApiUrl string `json:"api_url"` // 认证服务API地址
	ApiKey string `json:"api_key"` // 认证服务API密钥（可选）

	// HTTP客户端配置
	Timeout         time.Duration `json:"timeout"`           // HTTP请求超时时间
	MaxIdleConns    int           `json:"max_idle_conns"`    // 最大空闲连接数
	IdleConnTimeout time.Duration `json:"idle_conn_timeout"` // 空闲连接超时时间

	// 缓存配置
	EnableCache  bool          `json:"enable_cache"`  // 是否启用认证结果缓存
	CacheTimeout time.Duration `json:"cache_timeout"` // 缓存过期时间
	CacheSize    int           `json:"cache_size"`    // 缓存大小限制

	// 重试配置
	MaxRetries    int           `json:"max_retries"`    // 最大重试次数
	RetryInterval time.Duration `json:"retry_interval"` // 重试间隔

	// 调试配置
	Debug bool `json:"debug"` // 是否启用调试日志
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		ApiUrl:          "http://localhost:8000/api/v1",
		ApiKey:          "",
		Timeout:         60 * time.Second,
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
		EnableCache:     true,
		CacheTimeout:    5 * time.Minute,
		CacheSize:       1000,
		MaxRetries:      3,
		RetryInterval:   1 * time.Second,
		Debug:           false,
	}
}

var Auth *AuthConfig

func parseAuth() {
	apiUrl := GetDefaultEnv("AUTH_API_URL", "http://localhost:8000/api/v1")
	apiKey := GetDefaultEnv("AUTH_API_KEY", "")

	Auth = DefaultAuthConfig()
	Auth.ApiUrl = apiUrl
	Auth.ApiKey = apiKey

	// 可以从环境变量读取其他配置
	if timeout := GetDefaultEnv("AUTH_TIMEOUT", ""); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			Auth.Timeout = d
		}
	}

	if authCacheEnabled := GetDefaultEnv("AUTH_CACHE_ENABLED", "false"); authCacheEnabled == "true" {
		Auth.EnableCache = true
	} else {
		Auth.EnableCache = false
	}

	if debug := GetDefaultEnv("AUTH_DEBUG", "false"); debug == "true" {
		Auth.Debug = true
	}
}

func init() {
	parseAuth()
}
