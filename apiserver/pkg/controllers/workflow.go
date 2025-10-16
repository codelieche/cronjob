package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WorkflowController 工作流控制器
type WorkflowController struct {
	controllers.BaseController
	service core.WorkflowService
}

// NewWorkflowController 创建WorkflowController实例
func NewWorkflowController(service core.WorkflowService) *WorkflowController {
	return &WorkflowController{
		service: service,
	}
}

// Create 创建工作流
// @Summary 创建工作流
// @Description 创建新的工作流模板
// @Tags workflows
// @Accept json
// @Produce json
// @Param workflow body forms.WorkflowCreateForm true "工作流创建表单"
// @Success 201 {object} core.Workflow "创建成功的工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 409 {object} core.ErrorResponse "工作流代码已存在"
// @Router /workflow/ [post]
// @Security BearerAuth
// @Security TeamAuth
func (controller *WorkflowController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.WorkflowCreateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 准备创建对象
	workflow := form.ToWorkflow()

	// 🔥 如果没有传递team_id，则使用当前用户的team_id
	if workflow.TeamID == nil {
		if teamID, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
			if teamIDStr, ok := teamID.(string); ok && teamIDStr != "" {
				if parsedTeamID, err := uuid.Parse(teamIDStr); err == nil {
					workflow.TeamID = &parsedTeamID
				}
			}
		}
	}

	// 4. 调用服务创建工作流
	if err := controller.service.Create(c.Request.Context(), workflow); err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, workflow)
}

// Find 获取工作流信息
// @Summary 根据ID获取工作流
// @Description 根据工作流ID获取详细信息
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} core.Workflow "工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/ [get]
// @Security BearerAuth
func (controller *WorkflowController) Find(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	workflow, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	controller.HandleOK(c, workflow)
}

// FindByCode 根据Code获取工作流
// @Summary 根据Code获取工作流
// @Description 根据工作流Code获取详细信息（团队内唯一）
// @Tags workflows
// @Accept json
// @Produce json
// @Param code path string true "工作流Code"
// @Success 200 {object} core.Workflow "工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/by-code/{code}/ [get]
// @Security BearerAuth
// @Security TeamAuth
func (controller *WorkflowController) FindByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 获取team_id
	var teamID uuid.UUID
	if teamIDStr, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
		if teamIDValue, ok := teamIDStr.(string); ok && teamIDValue != "" {
			parsedTeamID, err := uuid.Parse(teamIDValue)
			if err != nil {
				controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
				return
			}
			teamID = parsedTeamID
		}
	}

	if teamID == uuid.Nil {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	workflow, err := controller.service.FindByCode(c.Request.Context(), teamID, code)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	controller.HandleOK(c, workflow)
}

// Update 更新工作流
// @Summary 更新工作流
// @Description 更新工作流信息
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param workflow body forms.WorkflowUpdateForm true "工作流更新表单"
// @Success 200 {object} core.Workflow "更新后的工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Failure 409 {object} core.ErrorResponse "工作流代码冲突"
// @Router /workflow/{id}/ [put]
// @Security BearerAuth
func (controller *WorkflowController) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 处理表单
	var form forms.WorkflowUpdateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 转换为Workflow对象
	workflow := form.ToWorkflow(uuidID)

	// 调用服务更新工作流
	if err := controller.service.Update(c.Request.Context(), workflow); err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 返回更新后的工作流
	updatedWorkflow, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		controller.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	controller.HandleOK(c, updatedWorkflow)
}

// Delete 删除工作流
// @Summary 删除工作流
// @Description 删除指定的工作流（软删除）
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 204 "删除成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/ [delete]
// @Security BearerAuth
func (controller *WorkflowController) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	if err := controller.service.Delete(c.Request.Context(), id); err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	controller.HandleNoContent(c)
}

// List 查询工作流列表
// @Summary 查询工作流列表
// @Description 查询工作流列表，支持过滤、搜索、排序和分页
// @Tags workflows
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param team_id query string false "团队ID"
// @Param project query string false "项目名称"
// @Param code query string false "工作流代码"
// @Param name query string false "工作流名称（精确匹配）"
// @Param name__contains query string false "工作流名称（模糊匹配）"
// @Param is_active query boolean false "是否激活"
// @Param search query string false "搜索关键字（名称/描述/代码）"
// @Param ordering query string false "排序字段（支持：name, code, created_at, updated_at, execute_count, last_execute_at，前缀-表示降序）" default("-created_at")
// @Success 200 {object} types.ResponseList "工作流列表"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /workflow/ [get]
// @Security BearerAuth
func (controller *WorkflowController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		{QueryKey: "id", Column: "id", Op: filters.FILTER_EQ},
		{QueryKey: "team_id", Column: "team_id", Op: filters.FILTER_EQ},
		{QueryKey: "project", Column: "project", Op: filters.FILTER_EQ},
		{QueryKey: "code", Column: "code", Op: filters.FILTER_EQ},
		{QueryKey: "name", Column: "name", Op: filters.FILTER_EQ},
		{QueryKey: "name__contains", Column: "name", Op: filters.FILTER_CONTAINS},
		{QueryKey: "is_active", Column: "is_active", Op: filters.FILTER_EQ},
		{QueryKey: "deleted", Column: "deleted", Op: filters.FILTER_EQ},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description", "code"}

	// 4. 定义排序字段
	orderingFields := []string{"name", "code", "created_at", "updated_at", "execute_count", "last_execute_at", "is_active"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员查看所有团队数据）
	filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, true)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取工作流列表
	workflows, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取总数
	count, err := controller.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 9. 返回列表响应
	result := &types.ResponseList{
		Results:  workflows,
		Count:    count,
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
	}
	controller.HandleOK(c, result)
}

// ToggleActive 切换激活状态
// @Summary 切换工作流激活状态
// @Description 切换工作流的激活状态
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} core.Workflow "更新后的工作流信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/toggle-active/ [post]
// @Security BearerAuth
func (controller *WorkflowController) ToggleActive(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	workflow, err := controller.service.ToggleActive(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	controller.HandleOK(c, workflow)
}

// GetStatistics 获取工作流统计信息
// @Summary 获取工作流统计信息
// @Description 获取工作流的执行统计信息
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Success 200 {object} map[string]interface{} "统计信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/statistics/ [get]
// @Security BearerAuth
func (controller *WorkflowController) GetStatistics(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	stats, err := controller.service.GetStatistics(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	controller.HandleOK(c, stats)
}
