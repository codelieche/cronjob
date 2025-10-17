package core

import "context"

// PermissionItem 权限项定义
type PermissionItem struct {
	Code        string `json:"code" binding:"required"` // 权限代码（不含系统前缀）
	Name        string `json:"name" binding:"required"` // 权限名称
	Description string `json:"description"`             // 权限描述
}

// RoleItem 角色项定义
type RoleItem struct {
	Code        string   `json:"code" binding:"required"`        // 角色代码（不含系统前缀）
	Name        string   `json:"name" binding:"required"`        // 角色名称
	Description string   `json:"description"`                    // 角色描述
	Permissions []string `json:"permissions" binding:"required"` // 权限代码列表（不含系统前缀）
}

// PlatformItem 平台项定义
type PlatformItem struct {
	Name           string `json:"name" binding:"required"`  // 平台名称
	Title          string `json:"title" binding:"required"` // 平台标题
	Icon           string `json:"icon"`                     // 平台图标
	Path           string `json:"path" binding:"required"`  // 路径
	Description    string `json:"description"`              // 描述
	DisplayOrder   int    `json:"display_order"`            // 显示顺序
	PermissionCode string `json:"permission_code"`          // 权限代码（不含系统前缀）
	IsMenu         bool   `json:"is_menu"`                  // 是否为菜单
	IsFrontend     bool   `json:"is_frontend"`              // 是否微前端
	Server         string `json:"server"`                   // 服务地址
	Container      string `json:"container"`                // 挂载点
	ActiveRule     string `json:"active_rule"`              // 激活规则
}

// PermissionRegistryRequest 权限注册请求
type PermissionRegistryRequest struct {
	SystemCode  string           `json:"system_code" binding:"required"` // 系统代码
	Permissions []PermissionItem `json:"permissions" binding:"required"` // 权限列表
	Roles       []RoleItem       `json:"roles" binding:"required"`       // 角色列表
}

// PlatformRegistryRequest 平台注册请求
type PlatformRegistryRequest struct {
	SystemCode string         `json:"system_code" binding:"required"` // 系统代码
	Platforms  []PlatformItem `json:"platforms" binding:"required"`   // 平台列表
}

// PermissionRegistryResponse 权限注册响应
type PermissionRegistryResponse struct {
	Message     string `json:"message"`
	SystemCode  string `json:"system_code"`
	Permissions int    `json:"permissions"`
	Roles       int    `json:"roles"`
}

// PlatformRegistryResponse 平台注册响应
type PlatformRegistryResponse struct {
	Message    string `json:"message"`
	SystemCode string `json:"system_code"`
	Platforms  int    `json:"platforms"`
}

// RegistryService 注册服务接口
type RegistryService interface {
	// RegisterPermissions 注册权限和角色到用户中心
	RegisterPermissions(ctx context.Context, req *PermissionRegistryRequest) (*PermissionRegistryResponse, error)

	// RegisterPlatforms 注册平台到用户中心
	RegisterPlatforms(ctx context.Context, req *PlatformRegistryRequest) (*PlatformRegistryResponse, error)
}
