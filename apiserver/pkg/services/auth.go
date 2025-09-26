// Package services 认证服务
//
// 提供认证相关的业务逻辑，包括：
// - 与usercenter认证服务的HTTP通信
// - 认证结果缓存管理
// - 认证性能优化（连接池、重试等）
package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// CacheEntry 缓存条目结构体
type CacheEntry struct {
	User      *core.AuthenticatedUser
	ExpiresAt time.Time
}

// IsExpired 检查缓存是否过期
func (c *CacheEntry) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// AuthService 认证服务接口
type AuthService interface {
	// Authenticate 执行用户认证
	Authenticate(ctx context.Context, authHeader string) *core.AuthResult

	// ClearCache 清空认证缓存
	ClearCache()

	// GetCacheStats 获取缓存统计信息
	GetCacheStats() map[string]interface{}
}

// authService 认证服务实现
type authService struct {
	config     *config.AuthConfig
	httpClient *http.Client
	cache      map[string]*CacheEntry // 简单的内存缓存
	cacheMutex sync.RWMutex           // 缓存读写锁
}

// NewAuthService 创建认证服务实例
func NewAuthService(cfg *config.AuthConfig) AuthService {
	if cfg == nil {
		cfg = config.DefaultAuthConfig()
	}

	// 创建HTTP客户端，配置连接池和超时
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.MaxIdleConns,
			IdleConnTimeout:     cfg.IdleConnTimeout,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: cfg.MaxIdleConns / 2,
		},
	}

	service := &authService{
		config:     cfg,
		httpClient: httpClient,
		cache:      make(map[string]*CacheEntry),
		cacheMutex: sync.RWMutex{},
	}

	logger.Info("认证服务初始化完成",
		zap.String("auth_url", cfg.ApiUrl),
		zap.Bool("cache_enabled", cfg.EnableCache))

	return service
}

// Authenticate 执行用户认证
func (s *authService) Authenticate(ctx context.Context, authHeader string) *core.AuthResult {
	startTime := time.Now()

	// 生成缓存键
	cacheKey := s.generateCacheKey(authHeader)

	// 尝试从缓存获取
	if s.config.EnableCache {
		if user := s.getFromCache(cacheKey); user != nil {
			if s.config.Debug {
				logger.Debug("认证结果来自缓存",
					zap.String("user_id", user.UserID),
					zap.String("auth_type", user.AuthType))
			}
			return &core.AuthResult{
				Success:   true,
				User:      user,
				FromCache: true,
				Duration:  time.Since(startTime),
			}
		}
	}

	// 执行实际的认证请求
	result := s.authenticateWithRetry(ctx, authHeader)
	result.Duration = time.Since(startTime)

	// 成功时存入缓存
	if result.Success && s.config.EnableCache && result.User != nil {
		s.setToCache(cacheKey, result.User)
	}

	return result
}

// authenticateWithRetry 带重试的认证请求
func (s *authService) authenticateWithRetry(ctx context.Context, authHeader string) *core.AuthResult {
	var lastErr error

	for i := 0; i <= s.config.MaxRetries; i++ {
		if i > 0 {
			// 重试前等待
			select {
			case <-time.After(s.config.RetryInterval):
			case <-ctx.Done():
				return &core.AuthResult{
					Success:      false,
					Error:        ctx.Err(),
					ErrorCode:    "CONTEXT_CANCELLED",
					ErrorMessage: "认证请求被取消",
				}
			}

			if s.config.Debug {
				logger.Debug("认证重试",
					zap.Int("attempt", i),
					zap.Error(lastErr))
			}
		}

		result := s.authenticateOnce(ctx, authHeader)
		if result.Success {
			return result
		}

		lastErr = result.Error

		// 某些错误不需要重试
		if s.shouldNotRetry(result.ErrorCode) {
			break
		}
	}

	return &core.AuthResult{
		Success:      false,
		Error:        lastErr,
		ErrorCode:    "MAX_RETRIES_EXCEEDED",
		ErrorMessage: fmt.Sprintf("认证失败，已重试%d次", s.config.MaxRetries),
	}
}

// authenticateOnce 执行单次认证请求
func (s *authService) authenticateOnce(ctx context.Context, authHeader string) *core.AuthResult {
	// 构建认证API URL
	authURL := strings.TrimSuffix(s.config.ApiUrl, "/") + "/auth/"

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", authURL, nil)
	if err != nil {
		return &core.AuthResult{
			Success:      false,
			Error:        err,
			ErrorCode:    "REQUEST_CREATE_FAILED",
			ErrorMessage: "创建HTTP请求失败",
		}
	}

	// 设置请求头
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CronJob-ApiServer/1.0")

	// 如果配置了API Key，添加到请求头
	if s.config.ApiKey != "" {
		req.Header.Set("X-API-Key", s.config.ApiKey)
	}

	// 发送请求
	httpResp, err := s.httpClient.Do(req)
	if err != nil {
		return &core.AuthResult{
			Success:      false,
			Error:        err,
			ErrorCode:    "HTTP_REQUEST_FAILED",
			ErrorMessage: "发送认证请求失败",
		}
	}
	defer httpResp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return &core.AuthResult{
			Success:      false,
			Error:        err,
			ErrorCode:    "RESPONSE_READ_FAILED",
			ErrorMessage: "读取响应体失败",
		}
	}

	// 解析响应
	return s.parseAuthResponse(httpResp.StatusCode, body)
}

// parseAuthResponse 解析认证响应
func (s *authService) parseAuthResponse(statusCode int, body []byte) *core.AuthResult {
	// 检查HTTP状态码
	if statusCode != http.StatusOK {
		return &core.AuthResult{
			Success:      false,
			ErrorCode:    fmt.Sprintf("HTTP_%d", statusCode),
			ErrorMessage: fmt.Sprintf("认证服务返回错误状态: %d", statusCode),
		}
	}

	// 解析JSON响应
	var apiResp core.Response
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return &core.AuthResult{
			Success:      false,
			Error:        err,
			ErrorCode:    "JSON_PARSE_FAILED",
			ErrorMessage: "解析认证响应失败",
		}
	}

	// 检查业务状态码
	if apiResp.Code != 0 {
		return &core.AuthResult{
			Success:      false,
			ErrorCode:    "AUTH_FAILED",
			ErrorMessage: apiResp.Message,
		}
	}

	// 解析用户数据
	user, err := s.parseUserData(apiResp.Data)
	if err != nil {
		return &core.AuthResult{
			Success:      false,
			Error:        err,
			ErrorCode:    "USER_DATA_PARSE_FAILED",
			ErrorMessage: "解析用户数据失败",
		}
	}

	return &core.AuthResult{
		Success: true,
		User:    user,
	}
}

// parseUserData 解析用户数据
func (s *authService) parseUserData(data interface{}) (*core.AuthenticatedUser, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("认证响应数据格式错误")
	}

	userMap, ok := dataMap["user"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("用户信息格式错误")
	}

	// 提取核心用户信息，使用类型断言并提供默认值
	userID, _ := userMap["id"].(string)
	username, _ := userMap["username"].(string)
	email, _ := userMap["email"].(string)
	phone, _ := userMap["phone"].(string)
	nickname, _ := userMap["nickname"].(string)
	isActive, _ := userMap["is_active"].(bool)
	isAdmin, _ := userMap["is_admin"].(bool)
	authType, _ := userMap["auth_type"].(string)
	currentTeam, _ := userMap["current_team"].(string)

	// API Key名称（可选，仅用于日志记录）
	apiKeyName, _ := userMap["api_key_name"].(string)

	// 验证必需字段
	if userID == "" || username == "" {
		return nil, fmt.Errorf("用户信息不完整：缺少用户ID或用户名")
	}

	// 构建优化后的用户对象
	user := &core.AuthenticatedUser{
		UserID:      userID,
		Username:    username,
		Email:       email,
		Phone:       phone,
		Nickname:    nickname,
		IsActive:    isActive,
		IsAdmin:     isAdmin,
		AuthType:    authType,
		CurrentTeam: currentTeam,
		ApiKeyName:  apiKeyName,
		CachedAt:    time.Now(),
	}

	return user, nil
}

// generateCacheKey 生成缓存键
func (s *authService) generateCacheKey(authHeader string) string {
	// 使用SHA256哈希生成缓存键，避免直接存储敏感信息，更安全
	hash := sha256.Sum256([]byte(authHeader))
	return fmt.Sprintf("auth_%x", hash)
}

// getFromCache 从缓存获取用户信息
func (s *authService) getFromCache(key string) *core.AuthenticatedUser {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	entry, exists := s.cache[key]
	if !exists || entry.IsExpired() {
		return nil
	}

	return entry.User
}

// setToCache 存储用户信息到缓存
func (s *authService) setToCache(key string, user *core.AuthenticatedUser) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// 检查缓存大小限制
	if len(s.cache) >= s.config.CacheSize {
		// 简单的缓存清理：删除过期条目
		s.cleanExpiredCache()

		// 如果仍然超出限制，删除一些条目（FIFO）
		if len(s.cache) >= s.config.CacheSize {
			count := 0
			for k := range s.cache {
				delete(s.cache, k)
				count++
				if count >= s.config.CacheSize/4 { // 删除25%的条目
					break
				}
			}
		}
	}

	s.cache[key] = &CacheEntry{
		User:      user,
		ExpiresAt: time.Now().Add(s.config.CacheTimeout),
	}
}

// cleanExpiredCache 清理过期的缓存条目
func (s *authService) cleanExpiredCache() {
	now := time.Now()
	for key, entry := range s.cache {
		if now.After(entry.ExpiresAt) {
			delete(s.cache, key)
		}
	}
}

// shouldNotRetry 判断是否不应该重试
func (s *authService) shouldNotRetry(errorCode string) bool {
	// 这些错误不应该重试
	noRetryErrors := []string{
		"AUTH_FAILED",            // 认证失败
		"USER_DATA_PARSE_FAILED", // 数据解析失败
		"JSON_PARSE_FAILED",      // JSON解析失败
		"CONTEXT_CANCELLED",      // 上下文取消
	}

	for _, code := range noRetryErrors {
		if errorCode == code {
			return true
		}
	}

	return false
}

// ClearCache 清空缓存
func (s *authService) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.cache = make(map[string]*CacheEntry)

	if s.config.Debug {
		logger.Debug("认证缓存已清空")
	}
}

// GetCacheStats 获取缓存统计信息
func (s *authService) GetCacheStats() map[string]interface{} {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	expired := 0
	now := time.Now()
	for _, entry := range s.cache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_entries":    len(s.cache),
		"expired_entries":  expired,
		"valid_entries":    len(s.cache) - expired,
		"cache_size_limit": s.config.CacheSize,
	}
}

// 全局认证服务实例（单例模式）
var (
	authServiceInstance AuthService
	authServiceOnce     sync.Once
)

// GetAuthService 获取认证服务实例（单例模式）
func GetAuthService() AuthService {
	authServiceOnce.Do(func() {
		authServiceInstance = NewAuthService(config.Auth)
	})
	return authServiceInstance
}
