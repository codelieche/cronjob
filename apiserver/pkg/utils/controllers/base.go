package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// BaseController Web控制器基础结构体
// 提供统一的HTTP响应处理、错误处理、分页解析和过滤器集成功能
// 所有具体的控制器都应该嵌入此结构体以获得基础功能
type BaseController struct {
}

// HandleOK 处理成功响应（200 OK）
// 返回标准格式的成功响应，code为0表示成功
func (controller *BaseController) HandleOK(c *gin.Context, data interface{}) {
	r := types.Response{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
	c.JSON(http.StatusOK, r)
}

// HandleNoContent 处理无内容响应（204 No Content）
// 用于删除操作等不需要返回数据的场景
func (controller *BaseController) HandleNoContent(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusNoContent)
}

// SetAuditLog 发送审计日志到后台系统
// 此方法用于记录用户操作日志，发送到审计系统进行后续分析
// 参数:
//   - c: Gin上下文，用于获取请求信息
//   - key: 审计日志的键名，用于标识操作类型
//   - data: 审计数据，包含操作详情
//   - marsharl: 是否对数据进行JSON序列化
//
// 注意: 此方法不返回HTTP响应，仅用于发送审计数据
func (controller *BaseController) SetAuditLog(c *gin.Context, key string, data interface{}, marsharl bool) {
	// 构建审计日志
	auditLog := &AuditLog{
		Action:     AuditAction(key),    // 将key转换为操作类型
		Resource:   c.Param("resource"), // 从路径参数获取资源类型
		ResourceID: c.Param("id"),       // 从路径参数获取资源ID
		UserID:     c.GetHeader("X-User-ID"),
		Username:   c.GetHeader("X-Username"),
		IP:         c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
		RequestID:  c.GetHeader("X-Request-ID"),
		Data:       make(map[string]interface{}),
		Level:      AuditLevelInfo,
		Message:    "用户操作审计日志",
		Success:    true,
	}

	// 处理数据
	if marsharl {
		// 如果需要序列化，将data转换为JSON
		if jsonData, err := json.Marshal(data); err == nil {
			auditLog.Data["raw_data"] = string(jsonData)
		}
	} else {
		// 直接使用原始数据
		auditLog.Data["raw_data"] = data
	}

	// 添加额外的请求信息
	auditLog.Data["method"] = c.Request.Method
	auditLog.Data["path"] = c.Request.URL.Path
	auditLog.Data["query"] = c.Request.URL.RawQuery

	// 异步发送审计日志
	service := GetAuditService()
	_ = service.SendAsync(c.Request.Context(), auditLog)
}

// HandleCreated 处理创建成功响应（201 Created）
// 用于资源创建操作的成功响应
func (controller *BaseController) HandleCreated(c *gin.Context, data interface{}) {
	r := types.Response{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
	c.JSON(http.StatusCreated, r)
}

// HandleError 处理通用错误响应
// 根据错误类型自动选择合适的HTTP状态码
// 如果错误是ErrNotFound，会自动调用Handle404
func (controller *BaseController) HandleError(c *gin.Context, err error, code int) {
	if err == core.ErrNotFound {
		controller.Handle404(c, err)
		return
	}

	r := types.Response{
		Code:    code,
		Message: err.Error(),
	}

	c.JSON(code, r)
}

// HandleError400 处理400错误响应（请求参数错误）
// 如果错误是ErrNotFound，会自动调用Handle404
func (controller *BaseController) HandleError400(c *gin.Context, err error) {
	if err == core.ErrNotFound {
		controller.Handle404(c, err)
		return
	}

	r := types.Response{
		Code:    http.StatusBadRequest,
		Message: err.Error(),
	}

	c.JSON(http.StatusBadRequest, r)
}

// Handle401 处理401错误响应（未授权）
// 用于token验证失败等认证相关错误
func (controller *BaseController) Handle401(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusUnauthorized,
		Message: err.Error(),
	}
	c.JSON(http.StatusUnauthorized, r)
}

// Handle403 处理403错误响应（禁止访问）
// 用于用户权限不足的场景
func (controller *BaseController) Handle403(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusForbidden,
		Message: err.Error(),
	}
	c.JSON(http.StatusForbidden, r)
}

// Handle404 处理404错误响应（资源不存在）
// 用于资源未找到的场景
func (controller *BaseController) Handle404(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusNotFound,
		Message: err.Error(),
	}
	c.JSON(http.StatusNotFound, r)
}

// HandleError500 处理500错误响应（内部服务器错误）
// 用于服务器内部错误，如数据库连接失败等
func (controller *BaseController) HandleError500(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
	}
	c.JSON(http.StatusInternalServerError, r)
}

// ParsePagination 解析分页参数
// 从HTTP请求的查询参数中提取分页信息，并进行合理性验证
// 返回: *types.Pagination - 包含页码和每页大小的分页对象
func (controller *BaseController) ParsePagination(c *gin.Context) *types.Pagination {
	// 解析页码参数，默认为1
	pageStr := c.DefaultQuery(pageConfig.PageQueryParam, "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1 // 解析失败时使用默认值
	}
	// 限制最大页码，防止恶意请求
	if pageConfig.MaxPage > 0 && page > pageConfig.MaxPage {
		page = pageConfig.MaxPage
	}

	// 解析每页大小参数，默认为10
	pageSizeStr := c.DefaultQuery(pageConfig.PageSizeQueryParam, "10")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		pageSize = 10 // 解析失败时使用默认值
	}

	// 限制最大每页大小，防止性能问题
	if pageConfig.MaxPageSize > 0 && pageSize > pageConfig.MaxPageSize {
		pageSize = pageConfig.MaxPageSize
	}

	return &types.Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

// FilterAction 创建过滤器动作组合
// 将过滤、搜索、排序功能组合成一个统一的过滤器动作列表
// 参数:
//   - c: Gin上下文，用于获取查询参数
//   - filterOptions: 过滤选项列表，定义可用的过滤字段和操作符
//   - searchFields: 搜索字段列表，用于多字段模糊搜索
//   - orderingFields: 排序字段列表，限制可排序的字段
//   - defaultOrdering: 默认排序规则，当没有指定排序时使用
//
// 返回: []filters.Filter - 过滤器动作列表，可直接应用到数据库查询
func (controller *BaseController) FilterAction(
	c *gin.Context, filterOptions []*filters.FilterOption,
	searchFields []string, orderingFields []string, defaultOrdering string) (filterActions []filters.Filter) {

	// 1. 创建字段过滤动作
	filterAction := filters.FromQueryGetFilterAction(c, filterOptions)
	if filterAction != nil {
		filterActions = append(filterActions, filterAction)
	}

	// 2. 创建搜索动作
	searchAction := filters.FromQueryGetSearchAction(c, searchFields)
	if searchAction != nil {
		filterActions = append(filterActions, searchAction)
	}

	// 3. 创建排序动作
	var orderingAction filters.Filter
	if orderingFields != nil && defaultOrdering != "" {
		// 使用默认排序规则
		orderingAction = filters.FromQueryGetOrderingActionWithDefault(c, orderingFields, defaultOrdering)
	} else {
		// 不使用默认排序规则
		orderingAction = filters.FromQueryGetOrderingAction(c, orderingFields)
	}
	if orderingAction != nil {
		filterActions = append(filterActions, orderingAction)
	}

	return filterActions
}

// LogAudit 记录审计日志的便捷方法
// 用于在控制器方法中记录用户操作
func (controller *BaseController) LogAudit(c *gin.Context, action AuditAction, resource string, resourceID string, data interface{}) {
	controller.SetAuditLog(c, string(action), data, true)
}

// LogCreateAudit 记录创建操作的审计日志
func (controller *BaseController) LogCreateAudit(c *gin.Context, resource string, resourceID string, data interface{}) {
	controller.LogAudit(c, AuditActionCreate, resource, resourceID, data)
}

// LogUpdateAudit 记录更新操作的审计日志
func (controller *BaseController) LogUpdateAudit(c *gin.Context, resource string, resourceID string, data interface{}) {
	controller.LogAudit(c, AuditActionUpdate, resource, resourceID, data)
}

// LogDeleteAudit 记录删除操作的审计日志
func (controller *BaseController) LogDeleteAudit(c *gin.Context, resource string, resourceID string, data interface{}) {
	controller.LogAudit(c, AuditActionDelete, resource, resourceID, data)
}

// LogReadAudit 记录读取操作的审计日志
func (controller *BaseController) LogReadAudit(c *gin.Context, resource string, resourceID string, data interface{}) {
	controller.LogAudit(c, AuditActionRead, resource, resourceID, data)
}

// ========== 认证相关辅助函数 ==========

// GetCurrentUser 从gin.Context中获取当前认证用户信息
// 返回完整的认证用户对象，包含所有用户信息
func (controller *BaseController) GetCurrentUser(c *gin.Context) (*core.AuthenticatedUser, bool) {
	if user, exists := c.Get(core.ContextKeyUser); exists {
		if authenticatedUser, ok := user.(*core.AuthenticatedUser); ok {
			return authenticatedUser, true
		}
	}
	return nil, false
}

// GetCurrentUserID 从gin.Context中获取当前用户ID
// 这是最常用的辅助函数，用于快速获取用户ID
func (controller *BaseController) GetCurrentUserID(c *gin.Context) (string, bool) {
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if id, ok := userID.(string); ok {
			return id, true
		}
	}
	return "", false
}

// GetCurrentTeam 从gin.Context中获取当前团队代码
// 返回用户当前选择的团队代码（字符串格式）
func (controller *BaseController) GetCurrentTeam(c *gin.Context) (string, bool) {
	if team, exists := c.Get(core.ContextKeyCurrentTeam); exists {
		if teamCode, ok := team.(string); ok {
			return teamCode, true
		}
	}
	return "", false
}

// GetCurrentTeamID 从gin.Context中获取当前团队ID
// 返回用户当前选择的团队ID（UUID字符串格式）
func (controller *BaseController) GetCurrentTeamID(c *gin.Context) (string, bool) {
	if teamID, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
		if id, ok := teamID.(string); ok {
			return id, true
		}
	}
	return "", false
}

// IsAuthenticated 检查当前请求是否已认证
// 返回true表示用户已通过认证，false表示未认证
func (controller *BaseController) IsAuthenticated(c *gin.Context) bool {
	if authenticated, exists := c.Get(core.ContextKeyIsAuthenticated); exists {
		if isAuth, ok := authenticated.(bool); ok {
			return isAuth
		}
	}
	return false
}

// IsAdmin 检查当前用户是否为管理员
// 返回true表示用户是管理员，false表示普通用户
func (controller *BaseController) IsAdmin(c *gin.Context) bool {
	if admin, exists := c.Get(core.ContextKeyIsAdmin); exists {
		if isAdmin, ok := admin.(bool); ok {
			return isAdmin
		}
	}
	return false
}

// GetUserTeamIDs 获取用户的团队ID列表
// 返回用户有权限访问的团队ID列表
func (controller *BaseController) GetUserTeamIDs(c *gin.Context) ([]string, bool) {
	if teamIDs, exists := c.Get("user_team_ids"); exists {
		if ids, ok := teamIDs.([]string); ok {
			return ids, true
		}
	}
	return nil, false
}

// IsUserInTeam 检查用户是否在指定团队中
// 管理员可以访问任意团队，普通用户只能访问自己加入的团队
func (controller *BaseController) IsUserInTeam(c *gin.Context, teamID string) bool {
	// 检查是否是管理员
	if controller.IsAdmin(c) {
		return true // 管理员可以访问任意团队
	}

	// 检查用户是否在该团队中
	userTeamIDs, exists := controller.GetUserTeamIDs(c)
	if !exists {
		return false
	}

	for _, id := range userTeamIDs {
		if id == teamID {
			return true
		}
	}
	return false
}

// ValidateResourceAccess 验证用户是否有权限访问指定资源
// 通用的权限验证方法，可用于任何需要验证用户权限的资源
func (controller *BaseController) ValidateResourceAccess(c *gin.Context, resourceUserID string, resourceTeamID *uuid.UUID) error {
	user, exists := controller.GetCurrentUser(c)
	if !exists {
		return errors.New("用户未认证")
	}

	// 1. 检查是否是资源的创建者
	if resourceUserID == user.UserID {
		return nil
	}

	// 2. 检查是否是管理员
	if user.IsAdmin {
		return nil
	}

	// 3. 检查是否在同一团队中
	if resourceTeamID != nil {
		if controller.IsUserInTeam(c, resourceTeamID.String()) {
			return nil
		}
	}

	return errors.New("无权限访问此资源")
}

// ValidateTeamAccess 验证团队访问权限
// 验证用户是否有权限访问指定的团队资源
func (controller *BaseController) ValidateTeamAccess(c *gin.Context, teamID *uuid.UUID) error {
	if teamID == nil {
		return nil // 个人数据，无需团队验证
	}

	if !controller.IsUserInTeam(c, teamID.String()) {
		return core.ErrForbidden
	}
	return nil
}

// ValidateTeamIDChange 验证 TeamID 修改权限
// 验证用户是否有权限将资源的团队归属修改为指定团队
func (controller *BaseController) ValidateTeamIDChange(c *gin.Context, newTeamID *uuid.UUID) error {
	if newTeamID == nil {
		return nil // 设置为个人数据，允许
	}

	if !controller.IsUserInTeam(c, newTeamID.String()) {
		return core.ErrForbidden
	}

	return nil
}

// AppendTeamFilterWithOptions 添加团队过滤器（包含权限验证和选项控制）
// 根据当前用户的团队权限，动态添加数据过滤条件
// 参数:
//   - c: Gin上下文
//   - filterActions: 现有的过滤器动作列表
//   - allowViewAll: 是否允许查看所有团队数据
func (controller *BaseController) AppendTeamFilterWithOptions(c *gin.Context, filterActions []filters.Filter, allowViewAll bool) []filters.Filter {
	// 🔥 检查是否请求查看跨团队数据
	if allowViewAll {
		viewAllTeams := c.Query("view_all_teams")
		if viewAllTeams == "true" {
			if controller.IsAdmin(c) {
				// 管理员可以查看所有团队数据，不添加任何团队过滤器
				logger.Info("管理员查看所有团队数据",
					zap.String("user_id", func() string {
						if userID, exists := controller.GetCurrentUserID(c); exists {
							return userID
						}
						return "unknown"
					}()))
				return filterActions
			} else {
				// 普通用户查看自己所属的所有团队数据（包括个人数据）
				return controller.AppendUserTeamsFilter(c, filterActions)
			}
		}
	}

	// 应用标准的团队过滤器（当前团队或个人数据）
	return controller.AppendTeamFilter(c, filterActions)
}

// AppendTeamFilter 添加团队过滤器（包含权限验证）
// 根据当前用户的团队权限，动态添加数据过滤条件
func (controller *BaseController) AppendTeamFilter(c *gin.Context, filterActions []filters.Filter) []filters.Filter {
	var teamFilter filters.Filter

	if currentTeamID, exists := controller.GetCurrentTeamID(c); exists && currentTeamID != "" {
		// 🔥 验证用户是否有权限访问该团队
		if !controller.IsUserInTeam(c, currentTeamID) {
			// 用户无权限访问该团队，返回空结果的过滤器
			noAccessFilter := &filters.FilterOption{
				Column: "id",
				Value:  "impossible-uuid-no-access", // 确保查询不到任何结果
				Op:     filters.FILTER_EQ,
			}
			teamFilter = noAccessFilter
		} else {
			// 查询团队数据
			teamFilterOption := &filters.FilterOption{
				Column: "team_id",
				Value:  currentTeamID,
				Op:     filters.FILTER_EQ,
			}
			teamFilter = teamFilterOption
		}
	} else {
		// 查询个人数据（team_id 为 null）
		personalFilterOption := &filters.FilterOption{
			Column: "team_id",
			Value:  nil,
			Op:     filters.FILTER_IS_NULL,
		}
		teamFilter = personalFilterOption
	}

	return append(filterActions, teamFilter)
}

// AppendUserTeamsFilter 添加用户所属团队过滤器
// 查询用户有权限访问的所有团队数据
func (controller *BaseController) AppendUserTeamsFilter(c *gin.Context, filterActions []filters.Filter) []filters.Filter {
	// 获取用户的团队ID列表
	userTeamIDs, exists := controller.GetUserTeamIDs(c)
	if !exists || len(userTeamIDs) == 0 {
		// 用户没有团队，只查询个人数据
		personalFilterOption := &filters.FilterOption{
			Column: "team_id",
			Value:  nil,
			Op:     filters.FILTER_IS_NULL,
		}
		logger.Info("用户查看个人数据（无团队）",
			zap.String("user_id", func() string {
				if userID, exists := controller.GetCurrentUserID(c); exists {
					return userID
				}
				return "unknown"
			}()))
		return append(filterActions, personalFilterOption)
	}

	// 🔥 简单直接：使用 FILTER_IN 查询用户所属的团队数据
	// 将 []string 转换为 []interface{}
	var teamIDsInterface []interface{}
	for _, teamID := range userTeamIDs {
		teamIDsInterface = append(teamIDsInterface, teamID)
	}

	teamFilterOption := &filters.FilterOption{
		Column: "team_id",
		Value:  teamIDsInterface,
		Op:     filters.FILTER_IN,
	}

	logger.Info("用户查看所属团队数据",
		zap.String("user_id", func() string {
			if userID, exists := controller.GetCurrentUserID(c); exists {
				return userID
			}
			return "unknown"
		}()),
		zap.Strings("team_ids", userTeamIDs),
		zap.Int("team_count", len(userTeamIDs)))

	return append(filterActions, teamFilterOption)
}

// ========== 权限检查相关方法 ==========

// GetUserPermissions 从 gin.Context 中获取用户权限列表
func (controller *BaseController) GetUserPermissions(c *gin.Context) ([]string, bool) {
	if permissions, exists := c.Get(core.ContextKeyPermissions); exists {
		if perms, ok := permissions.([]string); ok {
			return perms, true
		}
	}
	return nil, false
}

// GetUserRoles 从 gin.Context 中获取用户角色列表
func (controller *BaseController) GetUserRoles(c *gin.Context) ([]string, bool) {
	if roles, exists := c.Get(core.ContextKeyRoles); exists {
		if roleList, ok := roles.([]string); ok {
			return roleList, true
		}
	}
	return nil, false
}

// GetUserProjects 从 gin.Context 中获取用户项目列表
func (controller *BaseController) GetUserProjects(c *gin.Context) ([]string, bool) {
	if projects, exists := c.Get(core.ContextKeyProjects); exists {
		if projectList, ok := projects.([]string); ok {
			return projectList, true
		}
	}
	return nil, false
}

// CheckPermissionByType 通用权限检查方法
// 参数:
//   - c: Gin上下文
//   - checkType: 检查类型（"permission", "role", "project"）
//   - checkValue: 要检查的值
//
// 返回: bool - 是否有权限
func (controller *BaseController) CheckPermissionByType(c *gin.Context, checkType, checkValue string) bool {
	// 管理员拥有所有权限
	if controller.IsAdmin(c) {
		return true
	}

	switch checkType {
	case "permission":
		return controller.CheckPermission(c, checkValue)
	case "role":
		return controller.CheckRole(c, checkValue)
	case "project":
		return controller.CheckProject(c, checkValue)
	default:
		return false
	}
}

// CheckPermission 检查用户是否具有指定权限
func (controller *BaseController) CheckPermission(c *gin.Context, permission string) bool {
	// 管理员拥有所有权限
	if controller.IsAdmin(c) {
		return true
	}

	permissions, exists := controller.GetUserPermissions(c)
	if !exists {
		return false
	}

	for _, perm := range permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// CheckRole 检查用户是否具有指定角色
func (controller *BaseController) CheckRole(c *gin.Context, role string) bool {
	// 管理员拥有所有角色
	if controller.IsAdmin(c) {
		return true
	}

	roles, exists := controller.GetUserRoles(c)
	if !exists {
		return false
	}

	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// CheckProject 检查用户是否在指定项目中
func (controller *BaseController) CheckProject(c *gin.Context, project string) bool {
	// 管理员可以访问所有项目
	if controller.IsAdmin(c) {
		return true
	}

	projects, exists := controller.GetUserProjects(c)
	if !exists {
		return false
	}

	for _, proj := range projects {
		if proj == project {
			return true
		}
	}
	return false
}

// RequirePermission 要求指定权限，如果没有则返回403错误
func (controller *BaseController) RequirePermission(c *gin.Context, permission string) bool {
	if !controller.CheckPermission(c, permission) {
		controller.Handle403(c, errors.New("权限不足：需要权限 "+permission))
		return false
	}
	return true
}

// RequireRole 要求指定角色，如果没有则返回403错误
func (controller *BaseController) RequireRole(c *gin.Context, role string) bool {
	if !controller.CheckRole(c, role) {
		controller.Handle403(c, errors.New("权限不足：需要角色 "+role))
		return false
	}
	return true
}

// RequireProject 要求指定项目权限，如果没有则返回403错误
func (controller *BaseController) RequireProject(c *gin.Context, project string) bool {
	if !controller.CheckProject(c, project) {
		controller.Handle403(c, errors.New("权限不足：需要项目权限 "+project))
		return false
	}
	return true
}
