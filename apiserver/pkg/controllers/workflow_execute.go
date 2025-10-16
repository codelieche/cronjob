package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WorkflowExecuteController 工作流执行控制器
type WorkflowExecuteController struct {
	controllers.BaseController
	service core.WorkflowExecuteService
}

// NewWorkflowExecuteController 创建WorkflowExecuteController实例
func NewWorkflowExecuteController(service core.WorkflowExecuteService) *WorkflowExecuteController {
	return &WorkflowExecuteController{
		service: service,
	}
}

// Find 获取工作流执行实例
// @Summary 根据ID获取工作流执行实例
// @Description 根据执行实例ID获取详细信息
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param id path string true "执行实例ID"
// @Param include_tasks query bool false "是否包含任务列表" default(false)
// @Success 200 {object} core.WorkflowExecute "执行实例信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "执行实例不存在"
// @Router /workflow-execute/{id}/ [get]
// @Security BearerAuth
func (controller *WorkflowExecuteController) Find(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 🔥 检查是否需要包含任务列表
	includeTasks := c.Query("include_tasks") == "true"

	execute, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 🔥 如果需要包含任务列表，则查询并附加
	if includeTasks {
		tasks, err := controller.service.GetTasksByExecuteID(c.Request.Context(), id)
		if err != nil {
			// 任务查询失败不影响主数据返回，只记录日志
			// 但还是要设置为空列表
			execute.Tasks = []*core.Task{}
		} else {
			execute.Tasks = tasks
		}
	}

	controller.HandleOK(c, execute)
}

// List 查询工作流执行列表
// @Summary 查询工作流执行列表
// @Description 查询工作流执行列表，支持过滤、搜索、排序和分页
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param team_id query string false "团队ID"
// @Param workflow_id query string false "工作流ID"
// @Param project query string false "项目名称"
// @Param status query string false "执行状态（running/success/failed等）"
// @Param trigger_type query string false "触发类型（manual/api/webhook等）"
// @Param username query string false "触发用户"
// @Param search query string false "搜索关键字"
// @Param ordering query string false "排序字段（支持：created_at, time_start, time_end, status，前缀-表示降序）" default("-created_at")
// @Success 200 {object} types.ListResponse "执行实例列表"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /workflow-execute/ [get]
// @Security BearerAuth
func (controller *WorkflowExecuteController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		{QueryKey: "id", Column: "id", Op: filters.FILTER_EQ},
		{QueryKey: "team_id", Column: "team_id", Op: filters.FILTER_EQ},
		{QueryKey: "workflow_id", Column: "workflow_id", Op: filters.FILTER_EQ},
		{QueryKey: "project", Column: "project", Op: filters.FILTER_EQ}, // ⭐ 新增 project 过滤
		{QueryKey: "status", Column: "status", Op: filters.FILTER_EQ},
		{QueryKey: "trigger_type", Column: "trigger_type", Op: filters.FILTER_EQ},
		{QueryKey: "username", Column: "username", Op: filters.FILTER_EQ},
		{QueryKey: "worker_name", Column: "worker_name", Op: filters.FILTER_EQ},
		{QueryKey: "deleted", Column: "deleted", Op: filters.FILTER_EQ},
	}

	// 3. 定义搜索字段
	searchFields := []string{"username", "worker_name"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "time_start", "time_end", "status", "trigger_type"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员查看所有团队数据）
	filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, true)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取执行实例列表
	executes, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
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
		Results:  executes,
		Count:    count,
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
	}
	controller.HandleOK(c, result)
}

// ListByWorkflowID 根据WorkflowID查询执行列表
// @Summary 根据WorkflowID查询执行列表
// @Description 用于Workflow详情页的执行历史Tab
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} types.ResponseList "执行实例列表"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /workflow/{id}/executes/ [get]
// @Security BearerAuth
func (controller *WorkflowExecuteController) ListByWorkflowID(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 🔥 使用标准的分页参数解析（page, size）
	pagination := controller.ParsePagination(c)

	// 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 查询列表
	executes, err := controller.service.ListByWorkflowID(c.Request.Context(), workflowID, pagination.PageSize, offset)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 查询总数
	count, err := controller.service.CountByWorkflowID(c.Request.Context(), workflowID)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 🔥 返回标准的分页列表响应
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    count,
		Results:  executes,
	}
	controller.HandleOK(c, result)
}

// Cancel 取消工作流执行
// @Summary 取消工作流执行
// @Description 取消正在执行或待执行的工作流
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param id path string true "执行实例ID"
// @Success 200 {object} map[string]interface{} "取消成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "执行实例不存在"
// @Router /workflow-execute/{id}/cancel/ [post]
// @Security BearerAuth
func (controller *WorkflowExecuteController) Cancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 获取当前用户信息
	var userID *uuid.UUID
	var username string
	if userIDStr, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDValue, ok := userIDStr.(string); ok && userIDValue != "" {
			if parsedUserID, err := uuid.Parse(userIDValue); err == nil {
				userID = &parsedUserID
			}
		}
	}
	if usernameValue, exists := c.Get(core.ContextKeyUsername); exists {
		if usernameStr, ok := usernameValue.(string); ok {
			username = usernameStr
		}
	}

	// 调用服务取消执行
	if err := controller.service.Cancel(c.Request.Context(), id, userID, username); err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 返回成功响应
	controller.HandleOK(c, gin.H{
		"message": "工作流执行已取消",
		"id":      id,
	})
}

// Delete 删除工作流执行实例
// @Summary 删除工作流执行实例
// @Description 删除指定的工作流执行实例（软删除）
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param id path string true "执行实例ID"
// @Success 204 "删除成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "执行实例不存在"
// @Router /workflow-execute/{id}/ [delete]
// @Security BearerAuth
func (controller *WorkflowExecuteController) Delete(c *gin.Context) {
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

// Execute 触发工作流执行 ⭐
// @Summary 触发工作流执行
// @Description 触发工作流执行，创建所有Task并开始执行
// @Tags workflow-executes
// @Accept json
// @Produce json
// @Param id path string true "工作流ID"
// @Param body body map[string]interface{} false "执行参数（initial_variables, metadata_override）"
// @Success 201 {object} core.WorkflowExecute "执行实例信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "工作流不存在"
// @Router /workflow/{id}/execute/ [post]
// @Security BearerAuth
func (controller *WorkflowExecuteController) Execute(c *gin.Context) {
	// ========== Step 1: 解析 workflow_id ==========
	workflowIDStr := c.Param("id")
	if workflowIDStr == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	workflowID, err := uuid.Parse(workflowIDStr)
	if err != nil {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// ========== Step 2: 解析请求体 ==========
	var requestBody struct {
		InitialVariables map[string]interface{} `json:"initial_variables"`
		MetadataOverride map[string]interface{} `json:"metadata_override"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果没有请求体，不报错，使用空值
		requestBody.InitialVariables = make(map[string]interface{})
		requestBody.MetadataOverride = make(map[string]interface{})
	}

	// ========== Step 3: 获取当前用户信息 ==========
	var userID *uuid.UUID
	var username string

	if userIDStr, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDValue, ok := userIDStr.(string); ok && userIDValue != "" {
			if parsedUserID, err := uuid.Parse(userIDValue); err == nil {
				userID = &parsedUserID
			}
		}
	}

	if usernameValue, exists := c.Get(core.ContextKeyUsername); exists {
		if usernameStr, ok := usernameValue.(string); ok {
			username = usernameStr
		}
	}

	// ========== Step 4: 构建执行请求 ==========
	req := &core.ExecuteRequest{
		WorkflowID:       workflowID,
		TriggerType:      "manual", // TODO: 根据实际情况设置（manual/api/webhook）
		UserID:           userID,
		Username:         username,
		InitialVariables: requestBody.InitialVariables,
		MetadataOverride: requestBody.MetadataOverride,
	}

	// ========== Step 5: 调用服务执行 ==========
	workflowExec, err := controller.service.Execute(c.Request.Context(), req)
	if err != nil {
		if err == core.ErrNotFound {
			controller.HandleError(c, err, http.StatusNotFound)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// ========== Step 6: 返回执行实例 ==========
	controller.HandleCreated(c, workflowExec)
}
