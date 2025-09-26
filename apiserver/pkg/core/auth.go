package core

import (
	"time"
)

// AuthenticatedUser 认证用户信息结构体
// 包含从认证服务获取的核心用户信息，已优化为最小化数据传输
type AuthenticatedUser struct {
	// 核心认证字段
	UserID   string `json:"user_id"`   // 用户ID
	Username string `json:"username"`  // 用户名
	Email    string `json:"email"`     // 用户邮箱
	Phone    string `json:"phone"`     // 用户手机号（子系统可能需要）
	Nickname string `json:"nickname"`  // 用户昵称（用于友好显示）
	IsActive bool   `json:"is_active"` // 用户是否激活
	IsAdmin  bool   `json:"is_admin"`  // 是否管理员

	// 认证信息
	AuthType string `json:"auth_type"` // 认证类型（jwt或apikey）

	// 可选字段
	CurrentTeam string `json:"current_team,omitempty"` // 当前团队
	ApiKeyName  string `json:"api_key_name,omitempty"` // API Key名称（仅用于日志记录）

	// 缓存相关
	CachedAt time.Time `json:"cached_at,omitempty"` // 缓存时间
}

// IsAPIKeyAuth 判断是否为API Key认证
func (u *AuthenticatedUser) IsAPIKeyAuth() bool {
	return u.AuthType == "apikey"
}

// IsJWTAuth 判断是否为JWT认证
func (u *AuthenticatedUser) IsJWTAuth() bool {
	return u.AuthType == "jwt"
}

// HasApiKey 判断是否包含API Key信息
func (u *AuthenticatedUser) HasApiKey() bool {
	return u.ApiKeyName != ""
}

// GetDisplayName 获取用户显示名称
// 优先级：昵称 > 用户名 > 邮箱
func (u *AuthenticatedUser) GetDisplayName() string {
	if u.Nickname != "" {
		return u.Nickname
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// AuthResult 认证结果结构体
// 包含认证过程的详细信息和结果
type AuthResult struct {
	Success      bool               `json:"success"`       // 认证是否成功
	User         *AuthenticatedUser `json:"user"`          // 认证用户信息
	Error        error              `json:"error"`         // 错误信息
	ErrorCode    string             `json:"error_code"`    // 错误代码
	ErrorMessage string             `json:"error_message"` // 错误消息
	FromCache    bool               `json:"from_cache"`    // 是否来自缓存
	Duration     time.Duration      `json:"duration"`      // 认证耗时
}

// AuthContext Gin Context中的认证信息键名常量
// 统一管理Context中存储的认证相关键名，已优化移除不必要的键
const (
	ContextKeyUser            = "user"             // 完整用户对象
	ContextKeyUserID          = "user_id"          // 用户ID
	ContextKeyUsername        = "username"         // 用户名
	ContextKeyAuthType        = "auth_type"        // 认证类型
	ContextKeyCurrentTeam     = "current_team"     // 当前团队
	ContextKeyIsAuthenticated = "is_authenticated" // 是否已认证
	ContextKeyIsAdmin         = "is_admin"         // 是否管理员
	ContextKeyPhone           = "phone"            // 用户手机号
)
