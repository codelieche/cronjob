// Package middleware 认证辅助工具
//
// 提供便于开发人员使用的认证相关辅助函数和工具
// 包括路径配置、预定义中间件组合等
package middleware

import (
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/gin-gonic/gin"
)

// 预定义的跳过认证路径
var (
	// PublicPaths 公共路径，不需要认证
	PublicPaths = []string{
		"/",               // 根路径
		"/health",         // 健康检查（根级别）
		"/metrics",        // Prometheus监控指标
		"/api/v1/health/", // API健康检查
	}

	// WebSocketPaths WebSocket路径，有自己的认证机制（在Worker注册时验证）
	WebSocketPaths = []string{
		"/api/v1/ws/", // WebSocket连接
	}

	// WorkerPaths Worker节点专用路径，可能使用不同的认证方式
	WorkerPaths = []string{
		"/api/v1/worker/", // Worker相关接口
		"/api/v1/lock/",   // 分布式锁接口
	}
)

// AuthMiddlewareSkipPublic 跳过公共路径的认证中间件
// 适用于大部分业务接口，但跳过健康检查、监控等公共接口
func AuthMiddlewareSkipPublic() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.SkipPaths = PublicPaths
	return AuthMiddlewareWithConfig(cfg)
}

// AuthMiddlewareSkipWebSocket 跳过WebSocket路径的认证中间件
// 适用于需要WebSocket但不需要HTTP认证的场景
func AuthMiddlewareSkipWebSocket() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.SkipPaths = append(PublicPaths, WebSocketPaths...)
	return AuthMiddlewareWithConfig(cfg)
}

// AuthMiddlewareForWorkers 适用于Worker节点的认证中间件
// 跳过Worker专用路径，这些路径可能有自己的认证机制
func AuthMiddlewareForWorkers() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.SkipPaths = append(PublicPaths, WorkerPaths...)
	return AuthMiddlewareWithConfig(cfg)
}

// OptionalAuthMiddlewareSkipPublic 跳过公共路径的可选认证中间件
func OptionalAuthMiddlewareSkipPublic() gin.HandlerFunc {
	cfg := DefaultAuthMiddlewareConfig()
	cfg.Required = false
	cfg.SkipPaths = PublicPaths
	return AuthMiddlewareWithConfig(cfg)
}

// AuthMiddlewareGroup 认证中间件组合
// 提供常用的中间件组合，便于在路由中使用
type AuthMiddlewareGroup struct {
	// Standard 标准认证中间件（跳过公共路径）
	Standard gin.HandlerFunc

	// Optional 可选认证中间件（跳过公共路径）
	Optional gin.HandlerFunc

	// Admin 管理员认证中间件
	Admin gin.HandlerFunc

	// Strict 严格认证中间件（不跳过任何路径）
	Strict gin.HandlerFunc
}

// NewAuthMiddlewareGroup 创建认证中间件组合
func NewAuthMiddlewareGroup() *AuthMiddlewareGroup {
	return &AuthMiddlewareGroup{
		Standard: AuthMiddlewareSkipPublic(),
		Optional: OptionalAuthMiddlewareSkipPublic(),
		Admin:    AdminRequiredMiddleware(),
		Strict:   AuthMiddleware(),
	}
}

// RouteAuthConfig 路由认证配置
// 定义不同路由组的认证需求
type RouteAuthConfig struct {
	// 需要强制认证的路由前缀
	RequiredAuthPaths []string

	// 需要管理员权限的路由前缀
	AdminRequiredPaths []string

	// 可选认证的路由前缀
	OptionalAuthPaths []string

	// 完全跳过认证的路由前缀
	SkipAuthPaths []string
}

// DefaultRouteAuthConfig 默认路由认证配置
func DefaultRouteAuthConfig() *RouteAuthConfig {
	return &RouteAuthConfig{
		RequiredAuthPaths: []string{
			"/api/v1/user/",
			"/api/v1/cronjob/",
			"/api/v1/task/",
			"/api/v1/tasklog/",
			"/api/v1/category/",
		},
		AdminRequiredPaths: []string{
			"/api/v1/user/",   // 用户管理需要管理员权限
			"/api/v1/worker/", // Worker管理需要管理员权限（如果启用认证）
		},
		OptionalAuthPaths: []string{
			// 可以根据需要添加可选认证的路径
		},
		SkipAuthPaths: append(append(PublicPaths, WebSocketPaths...), WorkerPaths...),
	}
}

// ApplyAuthMiddleware 根据配置应用认证中间件到路由组
// 这是一个高级函数，可以根据路由路径自动应用相应的认证中间件
func ApplyAuthMiddleware(router *gin.RouterGroup, config *RouteAuthConfig) {
	if config == nil {
		config = DefaultRouteAuthConfig()
	}

	middlewareGroup := NewAuthMiddlewareGroup()

	// 为需要管理员权限的路径应用管理员中间件
	for _, path := range config.AdminRequiredPaths {
		subRouter := router.Group(path)
		subRouter.Use(middlewareGroup.Admin)
	}

	// 为需要强制认证的路径应用标准中间件
	for _, path := range config.RequiredAuthPaths {
		// 跳过已经应用管理员中间件的路径
		if !contains(config.AdminRequiredPaths, path) {
			subRouter := router.Group(path)
			subRouter.Use(middlewareGroup.Standard)
		}
	}

	// 为可选认证的路径应用可选中间件
	for _, path := range config.OptionalAuthPaths {
		subRouter := router.Group(path)
		subRouter.Use(middlewareGroup.Optional)
	}
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// AuthMiddlewareBuilder 认证中间件构建器
// 提供流式API来构建自定义的认证中间件
type AuthMiddlewareBuilder struct {
	config *AuthMiddlewareConfig
}

// NewAuthMiddlewareBuilder 创建认证中间件构建器
func NewAuthMiddlewareBuilder() *AuthMiddlewareBuilder {
	return &AuthMiddlewareBuilder{
		config: DefaultAuthMiddlewareConfig(),
	}
}

// Required 设置是否必需认证
func (b *AuthMiddlewareBuilder) Required(required bool) *AuthMiddlewareBuilder {
	b.config.Required = required
	return b
}

// SkipPaths 设置跳过认证的路径
func (b *AuthMiddlewareBuilder) SkipPaths(paths ...string) *AuthMiddlewareBuilder {
	b.config.SkipPaths = append(b.config.SkipPaths, paths...)
	return b
}

// SkipPublicPaths 跳过公共路径
func (b *AuthMiddlewareBuilder) SkipPublicPaths() *AuthMiddlewareBuilder {
	b.config.SkipPaths = append(b.config.SkipPaths, PublicPaths...)
	return b
}

// SkipWebSocketPaths 跳过WebSocket路径
func (b *AuthMiddlewareBuilder) SkipWebSocketPaths() *AuthMiddlewareBuilder {
	b.config.SkipPaths = append(b.config.SkipPaths, WebSocketPaths...)
	return b
}

// RequireAdmin 要求管理员权限
func (b *AuthMiddlewareBuilder) RequireAdmin() *AuthMiddlewareBuilder {
	b.config.RequireAdmin = true
	return b
}

// AllowedRoles 设置允许的角色
func (b *AuthMiddlewareBuilder) AllowedRoles(roles ...string) *AuthMiddlewareBuilder {
	b.config.AllowedRoles = append(b.config.AllowedRoles, roles...)
	return b
}

// CustomHandler 设置自定义处理函数
func (b *AuthMiddlewareBuilder) CustomHandler(handler func(*gin.Context, *core.AuthResult) bool) *AuthMiddlewareBuilder {
	b.config.CustomHandler = handler
	return b
}

// Build 构建认证中间件
func (b *AuthMiddlewareBuilder) Build() gin.HandlerFunc {
	return AuthMiddlewareWithConfig(b.config)
}

// 使用示例：
//
// // 基本使用
// router.Use(middleware.AuthMiddleware())
//
// // 跳过公共路径
// router.Use(middleware.AuthMiddlewareSkipPublic())
//
// // 可选认证
// router.Use(middleware.OptionalAuthMiddleware())
//
// // 管理员权限
// adminRouter := router.Group("/admin")
// adminRouter.Use(middleware.AdminRequiredMiddleware())
//
// // 使用构建器
// customAuth := middleware.NewAuthMiddlewareBuilder().
//     Required(true).
//     SkipPublicPaths().
//     RequireAdmin().
//     Build()
// router.Use(customAuth)
//
// // 使用中间件组合
// authGroup := middleware.NewAuthMiddlewareGroup()
// router.Use(authGroup.Standard)  // 标准认证
//
// // 自动应用中间件
// middleware.ApplyAuthMiddleware(router, middleware.DefaultRouteAuthConfig())
