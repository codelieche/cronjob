package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AIProviderController AI平台配置控制器
type AIProviderController struct {
	controllers.BaseController
	service *services.AIProviderService
}

// NewAIProviderController 创建AIProviderController实例
func NewAIProviderController(service *services.AIProviderService) *AIProviderController {
	return &AIProviderController{
		service: service,
	}
}

// Create 创建AI平台配置
// @Summary 创建AI平台配置
// @Description 创建新的AI平台配置，APIKey会自动加密存储
// @Tags ai-providers
// @Accept json
// @Produce json
// @Param provider body forms.AIProviderForm true "AI平台配置创建表单"
// @Success 201 {object} core.AIProvider "创建成功的AI平台配置"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /ai-providers/ [post]
// @Security BearerAuth
func (ctrl *AIProviderController) Create(c *gin.Context) {
	// 1. 解析表单
	var form forms.AIProviderForm
	if err := c.ShouldBind(&form); err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 获取当前用户TeamID
	currentUserTeamID := uuid.Nil
	if teamID, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
		if teamIDStr, ok := teamID.(string); ok && teamIDStr != "" {
			if parsedTeamID, err := uuid.Parse(teamIDStr); err == nil {
				currentUserTeamID = parsedTeamID
			}
		}
	}

	// 3. 获取当前用户ID（用于created_by）
	var createdBy *uuid.UUID
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				createdBy = &parsedUserID
			}
		}
	}

	// 4. 创建AIProvider对象
	provider := &core.AIProvider{
		Name:               form.Name,
		Description:        form.Description,
		ProviderType:       form.ProviderType,
		APIEndpoint:        form.APIEndpoint,
		APIKey:             form.APIKey, // 明文传入，Service层会加密
		DefaultModel:       form.DefaultModel,
		DefaultTemperature: form.DefaultTemperature,
		DefaultMaxTokens:   form.DefaultMaxTokens,
		DefaultTimeout:     form.DefaultTimeout,
		RateLimitRPM:       form.RateLimitRPM,
		RateLimitTPM:       form.RateLimitTPM,
		DailyBudget:        form.DailyBudget,
		Config:             form.Config,
		Enabled:            form.Enabled,
		TeamID:             form.TeamID,
		CreatedBy:          createdBy,
	}

	// 5. 调用Service创建（Service会自动填充team_id和加密APIKey）
	created, err := ctrl.service.Create(c.Request.Context(), provider, currentUserTeamID)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	ctrl.HandleCreated(c, created)
}

// List 获取AI平台配置列表
// @Summary 获取AI平台配置列表
// @Description 获取AI平台配置列表，支持分页和筛选
// @Tags ai-providers
// @Produce json
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页数量，默认20" default(20)
// @Param team_id query string false "团队ID过滤"
// @Param enabled query bool false "启用状态过滤"
// @Success 200 {object} core.Response "AI平台配置列表"
// @Router /ai-providers/ [get]
// @Security BearerAuth
func (ctrl *AIProviderController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := ctrl.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		{
			QueryKey: "team_id",
			Column:   "team_id",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "enabled",
			Column:   "enabled",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "provider_type",
			Column:   "provider_type",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义排序字段
	orderingFields := []string{"name", "created_at", "provider_type"}
	defaultOrdering := "-created_at"

	// 4. 获取过滤动作
	filterActions := ctrl.FilterAction(c, filterOptions, nil, orderingFields, defaultOrdering)

	// 5. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 6. 获取列表和总数
	providers, err := ctrl.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	count, err := ctrl.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 7. 返回分页响应
	ctrl.HandleOK(c, gin.H{
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"count":     count,
		"results":   providers,
	})
}

// Get 获取单个AI平台配置
// @Summary 获取单个AI平台配置
// @Description 根据ID获取AI平台配置详情（APIKey不返回）
// @Tags ai-providers
// @Produce json
// @Param id path string true "AI平台配置ID"
// @Success 200 {object} core.AIProvider "AI平台配置详情"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-providers/{id}/ [get]
// @Security BearerAuth
func (ctrl *AIProviderController) Get(c *gin.Context) {
	id := c.Param("id")
	provider, err := ctrl.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.HandleError(c, err, http.StatusNotFound)
		} else {
			ctrl.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	ctrl.HandleOK(c, provider)
}

// Update 更新AI平台配置
// @Summary 更新AI平台配置
// @Description 更新AI平台配置信息
// @Tags ai-providers
// @Accept json
// @Produce json
// @Param id path string true "AI平台配置ID"
// @Param provider body forms.AIProviderUpdateForm true "AI平台配置更新表单"
// @Success 200 {object} core.AIProvider "更新后的AI平台配置"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-providers/{id}/ [put]
// @Security BearerAuth
func (ctrl *AIProviderController) Update(c *gin.Context) {
	// 1. 获取ID
	id := c.Param("id")
	providerID, err := uuid.Parse(id)
	if err != nil {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 解析表单
	var form forms.AIProviderUpdateForm
	if err := c.ShouldBind(&form); err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 查找现有记录
	existing, err := ctrl.service.FindByID(c.Request.Context(), id)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusNotFound)
		return
	}

	// 4. 更新字段
	if form.Name != "" {
		existing.Name = form.Name
	}
	if form.Description != "" {
		existing.Description = form.Description
	}
	if form.APIEndpoint != "" {
		existing.APIEndpoint = form.APIEndpoint
	}
	if form.APIKey != "" {
		existing.APIKey = form.APIKey // Service层会加密
	}
	if form.DefaultModel != "" {
		existing.DefaultModel = form.DefaultModel
	}
	if form.DefaultTemperature != nil {
		existing.DefaultTemperature = form.DefaultTemperature
	}
	if form.DefaultMaxTokens != nil {
		existing.DefaultMaxTokens = form.DefaultMaxTokens
	}
	if form.DefaultTimeout != nil {
		existing.DefaultTimeout = form.DefaultTimeout
	}
	if form.RateLimitRPM != nil {
		existing.RateLimitRPM = form.RateLimitRPM
	}
	if form.RateLimitTPM != nil {
		existing.RateLimitTPM = form.RateLimitTPM
	}
	if form.DailyBudget != nil {
		existing.DailyBudget = form.DailyBudget
	}
	if form.Config != "" {
		existing.Config = form.Config
	}
	if form.Enabled != nil {
		existing.Enabled = form.Enabled
	}

	existing.ID = providerID

	// 5. 调用Service更新
	updated, err := ctrl.service.Update(c.Request.Context(), existing)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	ctrl.HandleOK(c, updated)
}

// Delete 删除AI平台配置
// @Summary 删除AI平台配置
// @Description 删除指定的AI平台配置
// @Tags ai-providers
// @Produce json
// @Param id path string true "AI平台配置ID"
// @Success 204 "删除成功"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-providers/{id}/ [delete]
// @Security BearerAuth
func (ctrl *AIProviderController) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := ctrl.service.DeleteByID(c.Request.Context(), id); err != nil {
		if err == core.ErrNotFound {
			ctrl.HandleError(c, err, http.StatusNotFound)
		} else {
			ctrl.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	c.Status(http.StatusNoContent)
}
