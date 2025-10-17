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

// AIAgentController AI Agent控制器
type AIAgentController struct {
	controllers.BaseController
	service *services.AIAgentService
}

// NewAIAgentController 创建AIAgentController实例
func NewAIAgentController(service *services.AIAgentService) *AIAgentController {
	return &AIAgentController{
		service: service,
	}
}

// Create 创建AI Agent
// @Summary 创建AI Agent
// @Description 创建新的AI Agent
// @Tags ai-agents
// @Accept json
// @Produce json
// @Param agent body forms.AIAgentForm true "AI Agent创建表单"
// @Success 201 {object} core.AIAgent "创建成功的AI Agent"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /ai-agents/ [post]
// @Security BearerAuth
func (ctrl *AIAgentController) Create(c *gin.Context) {
	// 1. 解析表单
	var form forms.AIAgentForm
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

	// 4. 创建AIAgent对象
	agent := &core.AIAgent{
		ProviderID:            form.ProviderID,
		Name:                  form.Name,
		Description:           form.Description,
		AgentType:             form.AgentType,
		Model:                 form.Model,
		Temperature:           form.Temperature,
		MaxTokens:             form.MaxTokens,
		Timeout:               form.Timeout,
		SystemPrompt:          form.SystemPrompt,
		PromptTemplate:        form.PromptTemplate,
		MaxContextLength:      form.MaxContextLength,
		IncludeHistory:        form.IncludeHistory,
		DecisionThreshold:     form.DecisionThreshold,
		AutoApproveConditions: form.AutoApproveConditions,
		AutoRejectConditions:  form.AutoRejectConditions,
		Config:                form.Config,
		Enabled:               form.Enabled,
		TeamID:                form.TeamID,
		CreatedBy:             createdBy,
	}

	// 5. 调用Service创建（Service会自动填充team_id）
	created, err := ctrl.service.Create(c.Request.Context(), agent, currentUserTeamID)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	ctrl.HandleCreated(c, created)
}

// List 获取AI Agent列表
// @Summary 获取AI Agent列表
// @Description 获取AI Agent列表，支持分页和筛选
// @Tags ai-agents
// @Produce json
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页数量，默认20" default(20)
// @Param team_id query string false "团队ID过滤"
// @Param provider_id query string false "Provider ID过滤"
// @Param agent_type query string false "Agent类型过滤"
// @Param enabled query bool false "启用状态过滤"
// @Success 200 {object} core.Response "AI Agent列表"
// @Router /ai-agents/ [get]
// @Security BearerAuth
func (ctrl *AIAgentController) List(c *gin.Context) {
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
			QueryKey: "provider_id",
			Column:   "provider_id",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "agent_type",
			Column:   "agent_type",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "enabled",
			Column:   "enabled",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义排序字段
	orderingFields := []string{"name", "created_at", "agent_type"}
	defaultOrdering := "-created_at"

	// 4. 获取过滤动作
	filterActions := ctrl.FilterAction(c, filterOptions, nil, orderingFields, defaultOrdering)

	// 5. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 6. 获取列表和总数
	agents, err := ctrl.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
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
		"results":   agents,
	})
}

// Get 获取单个AI Agent
// @Summary 获取单个AI Agent
// @Description 根据ID获取AI Agent详情
// @Tags ai-agents
// @Produce json
// @Param id path string true "AI Agent ID"
// @Success 200 {object} core.AIAgent "AI Agent详情"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-agents/{id}/ [get]
// @Security BearerAuth
func (ctrl *AIAgentController) Get(c *gin.Context) {
	id := c.Param("id")
	agent, err := ctrl.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.HandleError(c, err, http.StatusNotFound)
		} else {
			ctrl.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	ctrl.HandleOK(c, agent)
}

// Update 更新AI Agent
// @Summary 更新AI Agent
// @Description 更新AI Agent信息
// @Tags ai-agents
// @Accept json
// @Produce json
// @Param id path string true "AI Agent ID"
// @Param agent body forms.AIAgentUpdateForm true "AI Agent更新表单"
// @Success 200 {object} core.AIAgent "更新后的AI Agent"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-agents/{id}/ [put]
// @Security BearerAuth
func (ctrl *AIAgentController) Update(c *gin.Context) {
	// 1. 获取ID
	id := c.Param("id")
	agentID, err := uuid.Parse(id)
	if err != nil {
		ctrl.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 解析表单
	var form forms.AIAgentUpdateForm
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
	if form.Model != nil {
		existing.Model = form.Model
	}
	if form.Temperature != nil {
		existing.Temperature = form.Temperature
	}
	if form.MaxTokens != nil {
		existing.MaxTokens = form.MaxTokens
	}
	if form.Timeout != nil {
		existing.Timeout = form.Timeout
	}
	if form.SystemPrompt != "" {
		existing.SystemPrompt = form.SystemPrompt
	}
	if form.PromptTemplate != "" {
		existing.PromptTemplate = form.PromptTemplate
	}
	if form.MaxContextLength != nil {
		existing.MaxContextLength = form.MaxContextLength
	}
	if form.IncludeHistory != nil {
		existing.IncludeHistory = form.IncludeHistory
	}
	if form.DecisionThreshold != nil {
		existing.DecisionThreshold = form.DecisionThreshold
	}
	if form.AutoApproveConditions != "" {
		existing.AutoApproveConditions = form.AutoApproveConditions
	}
	if form.AutoRejectConditions != "" {
		existing.AutoRejectConditions = form.AutoRejectConditions
	}
	if form.Config != "" {
		existing.Config = form.Config
	}
	if form.Enabled != nil {
		existing.Enabled = form.Enabled
	}

	existing.ID = agentID

	// 5. 调用Service更新
	updated, err := ctrl.service.Update(c.Request.Context(), existing)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	ctrl.HandleOK(c, updated)
}

// Delete 删除AI Agent
// @Summary 删除AI Agent
// @Description 删除指定的AI Agent
// @Tags ai-agents
// @Produce json
// @Param id path string true "AI Agent ID"
// @Success 204 "删除成功"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /ai-agents/{id}/ [delete]
// @Security BearerAuth
func (ctrl *AIAgentController) Delete(c *gin.Context) {
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
