package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/credentials"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CredentialController 凭证控制器
type CredentialController struct {
	controllers.BaseController
	service core.CredentialService
}

// NewCredentialController 创建CredentialController实例
func NewCredentialController(service core.CredentialService) *CredentialController {
	return &CredentialController{
		service: service,
	}
}

// ListTypes 获取所有凭证类型
// @Summary 获取所有凭证类型
// @Description 获取系统支持的所有凭证类型定义
// @Tags credentials
// @Produce json
// @Success 200 {object} core.Response "凭证类型列表"
// @Router /credentials/types/ [get]
// @Security BearerAuth
func (controller *CredentialController) ListTypes(c *gin.Context) {
	credTypes := credentials.GetAll()

	result := make([]map[string]interface{}, 0, len(credTypes))
	for _, t := range credTypes {
		result = append(result, map[string]interface{}{
			"type":        t.GetType(),
			"label":       t.GetLabel(),
			"icon":        t.GetIcon(),
			"description": t.GetDescription(),
		})
	}

	controller.HandleOK(c, result)
}

// Create 创建凭证
// @Summary 创建凭证
// @Description 创建新的凭证，敏感字段自动加密
// @Tags credentials
// @Accept json
// @Produce json
// @Param credential body forms.CredentialForm true "凭证创建表单"
// @Success 201 {object} core.Credential "创建成功的凭证信息（敏感字段已脱敏）"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 403 {object} core.ErrorResponse "团队权限不足"
// @Router /credentials/ [post]
// @Security BearerAuth
// @Security TeamAuth
func (controller *CredentialController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.CredentialForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 将value转换为JSON
	valueJSON, err := json.Marshal(form.Value)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 创建凭证对象
	isActive := true
	if form.IsActive != nil {
		isActive = *form.IsActive
	}

	var teamID *uuid.UUID
	if form.TeamID != uuid.Nil {
		teamID = &form.TeamID
	}

	// 🔥 处理 metadata：如果为空字符串，则设置为 null（空 JSON 对象）
	metadata := form.Metadata
	if metadata == "" {
		metadata = "{}" // MySQL JSON 类型的 null 值
	}

	credential := &core.Credential{
		TeamID:      teamID,
		Category:    form.Category,
		Name:        form.Name,
		Description: form.Description,
		Project:     form.Project,      // 项目名称（可选）
		Value:       string(valueJSON), // 传递明文JSON，Service层会加密
		IsActive:    &isActive,         // 🔥 使用指针
		Metadata:    metadata,
	}

	// 🔥 如果没有传递team_id，则使用当前用户的team_id
	if credential.TeamID == nil || *credential.TeamID == uuid.Nil {
		if teamID, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
			if teamIDStr, ok := teamID.(string); ok && teamIDStr != "" {
				if parsedTeamID, err := uuid.Parse(teamIDStr); err == nil {
					credential.TeamID = &parsedTeamID
				}
			}
		}
	}

	// 🔥 自动设置创建人
	if credential.CreatedBy == nil {
		if userID, exists := c.Get(core.ContextKeyUserID); exists {
			if userIDStr, ok := userID.(string); ok && userIDStr != "" {
				if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
					credential.CreatedBy = &parsedUserID
				}
			}
		}
	}

	// 4. 调用服务创建凭证（Service会自动加密和脱敏）
	createdCredential, err := controller.service.Create(c.Request.Context(), credential)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdCredential)
}

// List 获取凭证列表
// @Summary 获取凭证列表
// @Description 获取凭证列表，支持分页和筛选
// @Tags credentials
// @Produce json
// @Param id query string false "凭证ID过滤"
// @Param category query string false "凭证类型过滤"
// @Param name query string false "凭证名称精确匹配"
// @Param name__contains query string false "凭证名称模糊搜索"
// @Param is_active query boolean false "是否启用过滤"
// @Param team_id query string false "团队ID过滤"
// @Param search query string false "搜索关键词（名称、描述）"
// @Param ordering query string false "排序字段（支持：name, created_at, updated_at, is_active）" default(-created_at)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param view_all_teams query boolean false "查看跨团队数据（管理员：所有团队，普通用户：自己所属团队）" example(true)
// @Success 200 {object} types.ResponseList "凭证列表（敏感字段已脱敏）"
// @Router /credentials/ [get]
// @Security BearerAuth
func (controller *CredentialController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		&filters.FilterOption{
			QueryKey: "id",
			Column:   "id",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "category",
			Column:   "category",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "name",
			Column:   "name",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "name__contains",
			Column:   "name",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "project",
			Column:   "project",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "project__contains",
			Column:   "project",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "is_active",
			Column:   "is_active",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "team_id",
			Column:   "team_id",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description", "project"}

	// 4. 定义排序字段
	orderingFields := []string{"name", "created_at", "updated_at", "is_active"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员查看所有团队数据）
	filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, true)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取凭证列表（Service会自动脱敏）
	credentials, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取凭证总数
	total, err := controller.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 9. 构建分页结果
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  credentials,
	}

	// 10. 返回结果
	controller.HandleOK(c, result)
}

// Find 获取凭证详情
// @Summary 根据ID获取凭证
// @Description 根据凭证ID获取详细信息（敏感字段已脱敏）
// @Tags credentials
// @Produce json
// @Param id path string true "凭证ID"
// @Success 200 {object} core.Credential "凭证信息（敏感字段已脱敏）"
// @Failure 404 {object} core.ErrorResponse "凭证不存在"
// @Router /credentials/{id}/ [get]
// @Security BearerAuth
func (controller *CredentialController) Find(c *gin.Context) {
	// 1. 获取凭证ID
	id := c.Param("id")

	// 2. 调用服务获取凭证（Service会自动脱敏）
	credential, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回凭证信息
	controller.HandleOK(c, credential)
}

// Update 更新凭证
// @Summary 更新凭证
// @Description 根据ID更新凭证信息
// @Tags credentials
// @Accept json
// @Produce json
// @Param id path string true "凭证ID"
// @Param credential body forms.CredentialUpdateForm true "凭证更新表单"
// @Success 200 {object} core.Credential "更新后的凭证信息（敏感字段已脱敏）"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "凭证不存在"
// @Router /credentials/{id}/ [put]
// @Security BearerAuth
func (controller *CredentialController) Update(c *gin.Context) {
	// 1. 获取凭证ID
	id := c.Param("id")

	// 2. 查询凭证
	credential, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单
	var form forms.CredentialUpdateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 更新字段
	if form.Name != "" {
		credential.Name = form.Name
	}
	if form.Description != "" {
		credential.Description = form.Description
	}
	// Project 字段允许清空（传空字符串）
	credential.Project = form.Project
	if form.Value != nil {
		// 将value转换为JSON
		valueJSON, err := json.Marshal(form.Value)
		if err != nil {
			controller.HandleError(c, err, http.StatusBadRequest)
			return
		}
		credential.Value = string(valueJSON) // 传递明文JSON，Service层会处理 ****** 并加密
		credential.Version++                 // 版本号递增
	}
	if form.IsActive != nil {
		credential.IsActive = form.IsActive // 🔥 直接赋值指针，无需解引用
	}
	// 🔥 metadata 在 Update 中不做处理，保持原值
	if form.Metadata != "" {
		credential.Metadata = form.Metadata
	}

	// 🔥 自动设置更新人
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				credential.UpdatedBy = &parsedUserID
			}
		}
	}

	// 5. 调用服务更新凭证
	updatedCredential, err := controller.service.Update(c.Request.Context(), credential)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	controller.HandleOK(c, updatedCredential)
}

// Delete 删除凭证
// @Summary 删除凭证
// @Description 根据ID删除凭证（软删除）
// @Tags credentials
// @Produce json
// @Param id path string true "凭证ID"
// @Success 200 {object} core.Response "删除成功"
// @Failure 404 {object} core.ErrorResponse "凭证不存在"
// @Router /credentials/{id}/ [delete]
// @Security BearerAuth
func (controller *CredentialController) Delete(c *gin.Context) {
	// 1. 获取凭证ID
	id := c.Param("id")

	// 2. 调用服务删除凭证
	if err := controller.service.DeleteByID(c.Request.Context(), id); err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// 3. 返回成功
	controller.HandleOK(c, gin.H{"message": "凭证删除成功"})
}

// Patch 动态更新凭证部分字段
// @Summary 动态更新凭证部分字段
// @Description 只更新提供的字段，支持更新：name, description, is_active, metadata, value 等
// @Tags credentials
// @Accept json
// @Produce json
// @Param id path string true "凭证ID"
// @Param updates body map[string]interface{} true "要更新的字段"
// @Success 200 {object} core.Response "更新成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "凭证不存在"
// @Router /credentials/{id}/ [patch]
// @Security BearerAuth
func (controller *CredentialController) Patch(c *gin.Context) {
	// 1. 获取凭证ID
	id := c.Param("id")

	// 2. 解析更新字段
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 🔥 自动设置更新人
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				updates["updated_by"] = parsedUserID.String()
			}
		}
	}

	// 3. 调用服务更新
	if err := controller.service.Patch(c.Request.Context(), id, updates); err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 4. 返回成功
	controller.HandleOK(c, gin.H{"message": "凭证更新成功"})
}

// Decrypt 解密凭证
// @Summary 解密凭证敏感字段
// @Description 解密凭证的敏感字段（需要特殊权限，操作会被记录）
// @Tags credentials
// @Produce json
// @Param id path string true "凭证ID"
// @Success 200 {object} map[string]interface{} "解密后的凭证内容"
// @Failure 403 {object} core.ErrorResponse "权限不足"
// @Failure 404 {object} core.ErrorResponse "凭证不存在"
// @Router /credentials/{id}/decrypt/ [post]
// @Security BearerAuth
func (controller *CredentialController) Decrypt(c *gin.Context) {
	// 1. 获取凭证ID
	id := c.Param("id")

	// 2. 调用服务解密凭证（获取完整信息）
	credentialData, err := controller.service.DecryptWithMetadata(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	// TODO: 记录解密操作日志

	// 3. 返回解密后的完整凭证信息（包括元数据）
	controller.HandleOK(c, credentialData)
}

// All 获取所有凭证（不分页）
// @Summary 获取所有凭证
// @Description 获取所有凭证，不分页，适用于凭证选择器等场景。支持团队过滤
// @Tags credentials
// @Accept json
// @Produce json
// @Param deleted query int false "是否已删除(1=已删除,0=未删除)" default(0)
// @Param category query string false "凭证类型过滤"
// @Param project query string false "项目名称过滤"
// @Param is_active query boolean false "是否启用过滤"
// @Param view_all_teams query boolean false "查看跨团队数据（管理员：所有团队，普通用户：自己所属团队）" example(false)
// @Success 200 {object} map[string]interface{} "凭证列表（敏感字段已脱敏）"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /credentials/all/ [get]
// @Security BearerAuth
func (controller *CredentialController) All(c *gin.Context) {
	// 解析deleted参数：1=已删除，0=未删除，默认为0
	deletedStr := c.DefaultQuery("deleted", "0")
	deleted := 0
	if deletedStr == "1" || deletedStr == "true" {
		deleted = 1
	}

	// 定义过滤选项
	filterOptions := []*filters.FilterOption{
		{
			Column: "deleted",
			Op:     filters.FILTER_EQ,
			Value:  deleted,
		},
		{
			QueryKey: "category",
			Column:   "category",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "project",
			Column:   "project",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "is_active",
			Column:   "is_active",
			Op:       filters.FILTER_EQ,
		},
	}

	// 定义搜索字段（空，不需要搜索）
	searchFields := []string{}

	// 定义排序字段
	orderingFields := []string{"name", "created_at", "updated_at"}
	defaultOrdering := "name" // 按名称排序

	// 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员查看所有团队数据）
	filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, true)

	// 获取所有凭证（设置较大的limit）
	credentials, err := controller.service.List(c.Request.Context(), 0, 10000, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 返回凭证列表（Service已经自动脱敏）
	controller.HandleOK(c, gin.H{
		"count":   len(credentials),
		"results": credentials,
	})
}
