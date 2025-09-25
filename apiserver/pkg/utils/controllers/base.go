package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
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
