// Package middleware 认证中间件包
//
// 提供灵活的认证中间件，支持多种认证方式和使用场景
// 设计理念：
// 1. 职责单一：只处理中间件逻辑，认证服务由services层提供
// 2. 灵活配置：支持不同的认证策略和路径配置
// 3. 统一接口：提供统一的错误处理和上下文设置
// 4. 开发友好：提供丰富的辅助函数
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthMiddlewareConfig 认证中间件配置
type AuthMiddlewareConfig struct {
	Required      bool                                      // 是否必需认证
	SkipPaths     []string                                  // 跳过认证的路径
	AllowedRoles  []string                                  // 允许的角色（可选）
	RequireAdmin  bool                                      // 是否需要管理员权限
	CustomHandler func(*gin.Context, *core.AuthResult) bool // 自定义处理函数
}

// DefaultAuthMiddlewareConfig 返回默认中间件配置
func DefaultAuthMiddlewareConfig() *AuthMiddlewareConfig {
	return &AuthMiddlewareConfig{
		Required:     true,
		SkipPaths:    []string{},
		AllowedRoles: []string{},
		RequireAdmin: false,
	}
}

// getAuthService 获取认证服务实例
// 使用services层提供的认证服务，实现关注点分离
func getAuthService() services.AuthService {
	return services.GetAuthService()
}

// AuthMiddleware 主认证中间件
// 这是推荐使用的认证中间件，提供强制认证功能
//
// 使用方式：
//
//	router.Use(middleware.AuthMiddleware())                    // 使用默认配置
//	router.Use(middleware.AuthMiddlewareWithConfig(config))   // 使用自定义配置
//
// 功能特性：
// - 强制认证：如果认证失败，请求将被拒绝
// - 支持JWT和API Key两种认证方式
// - 自动设置用户信息到Gin Context
// - 高性能缓存和连接池
// - 详细的错误日志和调试信息
func AuthMiddleware() gin.HandlerFunc {
	return AuthMiddlewareWithConfig(DefaultAuthMiddlewareConfig())
}

// AuthMiddlewareWithConfig 带配置的认证中间件
func AuthMiddlewareWithConfig(cfg *AuthMiddlewareConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultAuthMiddlewareConfig()
	}

	authService := getAuthService()

	return func(c *gin.Context) {
		startTime := time.Now()

		// 调试日志，确认中间件被执行
		logger.Debug("认证中间件开始执行",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))
		// 检查是否跳过认证
		if shouldSkipAuth(c, cfg.SkipPaths) {
			logger.Debug("跳过认证检查", zap.String("path", c.Request.URL.Path))
			c.Next()
			return
		}

		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			if cfg.Required {
				handleAuthError(c, &core.AuthResult{
					Success:      false,
					ErrorCode:    "MISSING_AUTH_HEADER",
					ErrorMessage: "缺少Authorization头",
				})
				return
			}
			// 可选认证时，继续处理请求
			c.Next()
			return
		}

		// 🔥 获取 X-TEAM-ID
		xTeamID := c.GetHeader("X-TEAM-ID")

		// 执行认证
		result := authService.Authenticate(c.Request.Context(), authHeader, xTeamID)

		// 记录认证耗时
		logger.Debug("认证完成",
			zap.Bool("success", result.Success),
			zap.Duration("duration", result.Duration),
			zap.Bool("from_cache", result.FromCache),
			zap.String("error_code", result.ErrorCode))

		// 处理认证结果
		if !result.Success {
			if cfg.Required {
				// 记录认证失败的详细信息
				logger.Warn("用户认证失败",
					zap.String("error_code", result.ErrorCode),
					zap.String("error_message", result.ErrorMessage),
					zap.String("client_ip", c.ClientIP()),
					zap.String("user_agent", c.GetHeader("User-Agent")),
					zap.Duration("duration", result.Duration))

				handleAuthError(c, result)
				return
			}
			// 可选认证时，继续处理请求
			c.Next()
			return
		}

		// 认证成功，检查权限
		if !checkPermissions(result.User, cfg) {
			handlePermissionError(c, result.User, cfg)
			return
		}

		// 自定义处理函数
		if cfg.CustomHandler != nil {
			if !cfg.CustomHandler(c, result) {
				return // 自定义处理函数已处理响应
			}
		}

		// 设置用户信息到Context
		setUserContext(c, result.User)

		// 记录认证成功信息
		logger.Debug("用户认证成功",
			zap.String("user_id", result.User.UserID),
			zap.String("username", result.User.Username),
			zap.String("auth_type", result.User.AuthType),
			zap.Duration("auth_duration", time.Since(startTime)))

		// 继续处理请求
		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件
// 如果提供了认证信息则验证，否则继续处理请求
//
// 使用场景：
// - 某些接口可以匿名访问，但认证后有更多功能
// - 需要根据认证状态返回不同内容的接口
//
// 使用方式：
//
//	router.Use(middleware.OptionalAuthMiddleware())
func OptionalAuthMiddleware() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.Required = false
	return AuthMiddlewareWithConfig(cfg)
}

// AdminRequiredMiddleware 管理员权限中间件
// 要求用户必须是管理员才能访问
func AdminRequiredMiddleware() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.RequireAdmin = true
	return AuthMiddlewareWithConfig(cfg)
}

// shouldSkipAuth 检查是否应该跳过认证
func shouldSkipAuth(c *gin.Context, skipPaths []string) bool {
	path := c.Request.URL.Path

	for _, skipPath := range skipPaths {
		// 修复路径匹配逻辑：
		// 1. 如果skipPath是"/"，只匹配根路径
		// 2. 其他情况使用前缀匹配
		if skipPath == "/" {
			if path == "/" {
				logger.Debug("路径匹配根路径跳过条件",
					zap.String("request_path", path))
				return true
			}
		} else if strings.HasPrefix(path, skipPath) {
			logger.Debug("路径匹配跳过条件",
				zap.String("request_path", path),
				zap.String("skip_path", skipPath))
			return true
		}
	}
	return false
}

// checkPermissions 检查用户权限
func checkPermissions(user *core.AuthenticatedUser, cfg *AuthMiddlewareConfig) bool {
	// 检查用户是否激活
	if !user.IsActive {
		return false
	}

	// 检查是否需要管理员权限
	if cfg.RequireAdmin && !user.IsAdmin {
		return false
	}

	// 检查角色权限（如果配置了允许的角色）
	if len(cfg.AllowedRoles) > 0 {
		// 这里可以根据实际需求实现角色检查逻辑
		// 目前简化处理：管理员可以访问所有角色限制的资源
		if !user.IsAdmin {
			// TODO: 实现具体的角色检查逻辑
			return false
		}
	}

	return true
}

// setUserContext 设置用户信息到Gin Context
func setUserContext(c *gin.Context, user *core.AuthenticatedUser) {
	// 设置完整的用户对象
	c.Set(core.ContextKeyUser, user)

	// 设置常用的快捷字段（已优化，移除不必要的字段）
	c.Set(core.ContextKeyUserID, user.UserID)
	c.Set(core.ContextKeyUsername, user.Username)
	c.Set(core.ContextKeyAuthType, user.AuthType)
	c.Set(core.ContextKeyCurrentTeam, user.CurrentTeam)
	c.Set(core.ContextKeyCurrentTeamID, user.CurrentTeamID)
	c.Set(core.ContextKeyIsAuthenticated, user.IsActive)
	c.Set(core.ContextKeyIsAdmin, user.IsAdmin)
	c.Set(core.ContextKeyPhone, user.Phone)

	// 🔥 设置权限相关字段
	c.Set(core.ContextKeyPermissions, user.Permissions)
	c.Set(core.ContextKeyRoles, user.Roles)
	c.Set(core.ContextKeyProjects, user.Projects)

	// 🔥 解析并设置用户的团队ID列表
	if user.Teams != "" {
		teamIDs := parseTeamIDs(user.Teams)
		c.Set("user_team_ids", teamIDs)
		logger.Debug("设置用户团队ID列表", zap.Strings("team_ids", teamIDs))
	}
}

// handleAuthError 处理认证错误
func handleAuthError(c *gin.Context, result *core.AuthResult) {
	// 根据错误类型返回不同的HTTP状态码
	statusCode := http.StatusUnauthorized

	switch result.ErrorCode {
	case "HTTP_REQUEST_FAILED", "RESPONSE_READ_FAILED":
		statusCode = http.StatusServiceUnavailable
	case "MAX_RETRIES_EXCEEDED":
		statusCode = http.StatusServiceUnavailable
	case "CONTEXT_CANCELLED":
		statusCode = http.StatusRequestTimeout
	}

	// 统一的错误响应格式
	c.JSON(statusCode, gin.H{
		"code":    statusCode,
		"message": result.ErrorMessage,
		"error":   result.ErrorCode,
	})
	c.Abort()
}

// handlePermissionError 处理权限错误
func handlePermissionError(c *gin.Context, user *core.AuthenticatedUser, cfg *AuthMiddlewareConfig) {
	var message string

	if !user.IsActive {
		message = "用户账户未激活"
	} else if cfg.RequireAdmin && !user.IsAdmin {
		message = "需要管理员权限"
	} else {
		message = "权限不足"
	}

	logger.Warn("用户权限不足",
		zap.String("user_id", user.UserID),
		zap.String("username", user.Username),
		zap.Bool("is_active", user.IsActive),
		zap.Bool("is_admin", user.IsAdmin),
		zap.Bool("require_admin", cfg.RequireAdmin),
		zap.String("path", c.Request.URL.Path))

	c.JSON(http.StatusForbidden, gin.H{
		"code":    http.StatusForbidden,
		"message": message,
		"error":   "PERMISSION_DENIED",
	})
	c.Abort()
}

// GetCurrentUser 从Gin Context获取当前用户信息
// 这是一个辅助函数，供控制器使用
func GetCurrentUser(c *gin.Context) (*core.AuthenticatedUser, bool) {
	if user, exists := c.Get(core.ContextKeyUser); exists {
		if authUser, ok := user.(*core.AuthenticatedUser); ok {
			return authUser, true
		}
	}
	return nil, false
}

// RequireAuth 检查是否已认证，如果未认证则返回错误
// 这是一个辅助函数，供控制器使用
func RequireAuth(c *gin.Context) (*core.AuthenticatedUser, bool) {
	user, exists := GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "需要认证",
			"error":   "AUTHENTICATION_REQUIRED",
		})
		return nil, false
	}
	return user, true
}

// RequireAdmin 检查是否为管理员，如果不是则返回错误
// 这是一个辅助函数，供控制器使用
func RequireAdmin(c *gin.Context) (*core.AuthenticatedUser, bool) {
	user, exists := RequireAuth(c)
	if !exists {
		return nil, false
	}

	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    http.StatusForbidden,
			"message": "需要管理员权限",
			"error":   "ADMIN_REQUIRED",
		})
		return nil, false
	}

	return user, true
}

// ClearAuthCache 清空认证缓存
// 这是一个管理函数，可以在需要时清空缓存
func ClearAuthCache() {
	authService := getAuthService()
	authService.ClearCache()
	logger.Info("认证缓存已清空")
}

// GetAuthCacheStats 获取认证缓存统计信息
func GetAuthCacheStats() map[string]interface{} {
	authService := getAuthService()
	return authService.GetCacheStats()
}

// parseTeamIDs 解析团队ID列表
// 🔥 核心优化：适配简化的团队ID列表格式
// 从 JSON 字符串中提取团队ID列表
// 简化前：[{"id":"uuid1"}, {"id":"uuid2"}] -> ["uuid1", "uuid2"]
// 简化后：["uuid1", "uuid2"] -> ["uuid1", "uuid2"]
func parseTeamIDs(teamsJSON string) []string {
	if teamsJSON == "" {
		return nil
	}

	// 🔥 尝试解析简化格式的团队ID列表（字符串数组）
	var teamIDs []string
	if err := json.Unmarshal([]byte(teamsJSON), &teamIDs); err == nil {
		return teamIDs
	}

	// 🔥 向后兼容：尝试解析旧格式的团队列表（对象数组）
	var teams []map[string]interface{}
	if err := json.Unmarshal([]byte(teamsJSON), &teams); err != nil {
		logger.Warn("解析团队列表失败", zap.Error(err), zap.String("teams_json", teamsJSON))
		return nil
	}

	var legacyTeamIDs []string
	for _, team := range teams {
		if id, ok := team["id"].(string); ok && id != "" {
			legacyTeamIDs = append(legacyTeamIDs, id)
		}
	}

	return legacyTeamIDs
}
