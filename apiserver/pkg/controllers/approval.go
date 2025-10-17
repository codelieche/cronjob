package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ApprovalController 审批控制器
type ApprovalController struct {
	controllers.BaseController
	service *services.ApprovalService
}

// NewApprovalController 创建ApprovalController实例
func NewApprovalController(service *services.ApprovalService) *ApprovalController {
	return &ApprovalController{
		service: service,
	}
}

// Create 创建审批
// @Summary 创建审批
// @Description 创建新的审批，team_id和created_by会自动填充
// @Tags approvals
// @Accept json
// @Produce json
// @Param approval body forms.ApprovalForm true "审批创建表单"
// @Success 201 {object} core.Approval "创建成功的审批"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Router /approvals/ [post]
// @Security BearerAuth
func (ctrl *ApprovalController) Create(c *gin.Context) {
	// 1. 解析表单
	var form forms.ApprovalForm
	if err := c.ShouldBind(&form); err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 获取当前用户信息
	currentUserID := uuid.Nil
	currentUserTeamID := uuid.Nil

	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				currentUserID = parsedUserID
			}
		}
	}

	if teamID, exists := c.Get(core.ContextKeyCurrentTeamID); exists {
		if teamIDStr, ok := teamID.(string); ok && teamIDStr != "" {
			if parsedTeamID, err := uuid.Parse(teamIDStr); err == nil {
				currentUserTeamID = parsedTeamID
			}
		}
	}

	// 3. 转换JSON字段为json.RawMessage
	userIDsJSON, _ := json.Marshal(form.UserIDs)
	aiAgentIDsJSON, _ := json.Marshal(form.AIAgentIDs)

	// 4. 设置默认超时时间
	timeout := form.Timeout
	if timeout == 0 {
		timeout = 3600 // 默认1小时
	}

	// 5. 处理JSON字段，空字符串转为有效的JSON
	contextJSON := json.RawMessage(form.Context)
	if form.Context == "" {
		contextJSON = json.RawMessage("{}")
	}
	metadataJSON := json.RawMessage(form.Metadata)
	if form.Metadata == "" {
		metadataJSON = json.RawMessage("{}")
	}

	// 6. 创建Approval对象
	approval := &core.Approval{
		TaskID:         form.TaskID,
		WorkflowExecID: form.WorkflowExecID,
		Title:          form.Title,
		Content:        form.Content,
		Context:        contextJSON,
		UserIDs:        userIDsJSON,
		AIAgentIDs:     aiAgentIDsJSON,
		RequireAll:     form.RequireAll,
		Timeout:        timeout,
		Metadata:       metadataJSON,
		TeamID:         form.TeamID,
		CreatedBy:      form.CreatedBy,
	}

	// 6. 调用Service创建（Service会自动填充team_id和created_by）
	created, err := ctrl.service.Create(c.Request.Context(), approval, currentUserID, currentUserTeamID)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 7. 返回成功响应
	ctrl.HandleCreated(c, created)
}

// List 获取审批列表
// @Summary 获取审批列表
// @Description 获取审批列表，支持分页和筛选
// @Tags approvals
// @Produce json
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页数量，默认20" default(20)
// @Param team_id query string false "团队ID过滤"
// @Param status query string false "状态过滤：pending/approved/rejected/timeout/cancelled"
// @Param task_id query string false "Task ID过滤"
// @Success 200 {object} core.Response "审批列表"
// @Router /approvals/ [get]
// @Security BearerAuth
func (ctrl *ApprovalController) List(c *gin.Context) {
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
			QueryKey: "status",
			Column:   "status",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "task_id",
			Column:   "task_id",
			Op:       filters.FILTER_EQ,
		},
		{
			QueryKey: "workflow_exec_id",
			Column:   "workflow_exec_id",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义排序字段
	orderingFields := []string{"created_at", "status", "title"}
	defaultOrdering := "-created_at"

	// 4. 获取过滤动作
	filterActions := ctrl.FilterAction(c, filterOptions, nil, orderingFields, defaultOrdering)

	// 5. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 6. 获取列表和总数
	approvals, err := ctrl.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
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
		"results":   approvals,
	})
}

// Get 获取单个审批
// @Summary 获取单个审批
// @Description 根据ID获取审批详情
// @Tags approvals
// @Produce json
// @Param id path string true "审批ID"
// @Success 200 {object} core.Approval "审批详情"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /approvals/{id}/ [get]
// @Security BearerAuth
func (ctrl *ApprovalController) Get(c *gin.Context) {
	id := c.Param("id")
	approval, err := ctrl.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.HandleError(c, err, http.StatusNotFound)
		} else {
			ctrl.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	ctrl.HandleOK(c, approval)
}

// HandleAction 处理审批操作（统一接口）
// @Summary 处理审批操作
// @Description 统一的审批操作接口，支持approve/reject/cancel操作
// @Tags approvals
// @Accept json
// @Produce json
// @Param id path string true "审批ID"
// @Param action body forms.ApprovalActionForm true "审批操作表单"
// @Success 200 {object} core.Response "操作成功"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /approvals/{id}/action/ [post]
// @Security BearerAuth
func (ctrl *ApprovalController) HandleAction(c *gin.Context) {
	// 1. 获取审批ID
	approvalID := c.Param("id")

	// 2. 解析操作表单
	var form forms.ApprovalActionForm
	if err := c.ShouldBind(&form); err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 获取当前用户ID
	currentUserID := uuid.Nil
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				currentUserID = parsedUserID
			}
		}
	}

	if currentUserID == uuid.Nil {
		ctrl.HandleError(c, core.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// 4. 调用Service处理操作
	err := ctrl.service.HandleAction(c.Request.Context(), approvalID, form.Action, form.Comment, currentUserID)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 返回成功响应
	ctrl.HandleOK(c, gin.H{
		"message": "操作成功",
		"action":  form.Action,
	})
}

// MyPending 查找我的待审批
// @Summary 查找我的待审批
// @Description 查找当前用户的待审批列表
// @Tags approvals
// @Produce json
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页数量，默认20" default(20)
// @Success 200 {object} core.Response "我的待审批列表"
// @Router /approvals/my/pending/ [get]
// @Security BearerAuth
func (ctrl *ApprovalController) MyPending(c *gin.Context) {
	// 1. 解析分页参数
	pagination := ctrl.ParsePagination(c)
	offset := (pagination.Page - 1) * pagination.PageSize
	pageSize := pagination.PageSize

	// 2. 获取当前用户ID
	currentUserID := uuid.Nil
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				currentUserID = parsedUserID
			}
		}
	}

	if currentUserID == uuid.Nil {
		ctrl.HandleError(c, core.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// 3. 查询我的待审批
	approvals, err := ctrl.service.FindMyPending(c.Request.Context(), currentUserID, offset, pageSize)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 4. 返回响应（简化版，不计算总数）
	ctrl.HandleOK(c, gin.H{
		"results": approvals,
		"count":   len(approvals),
	})
}

// MyCreated 查找我发起的审批
// @Summary 查找我发起的审批
// @Description 查找当前用户发起的审批列表
// @Tags approvals
// @Produce json
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页数量，默认20" default(20)
// @Success 200 {object} core.Response "我发起的审批列表"
// @Router /approvals/my/created/ [get]
// @Security BearerAuth
func (ctrl *ApprovalController) MyCreated(c *gin.Context) {
	// 1. 解析分页参数
	pagination := ctrl.ParsePagination(c)
	offset := (pagination.Page - 1) * pagination.PageSize
	pageSize := pagination.PageSize

	// 2. 获取当前用户ID
	currentUserID := uuid.Nil
	if userID, exists := c.Get(core.ContextKeyUserID); exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			if parsedUserID, err := uuid.Parse(userIDStr); err == nil {
				currentUserID = parsedUserID
			}
		}
	}

	if currentUserID == uuid.Nil {
		ctrl.HandleError(c, core.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// 3. 查询我发起的审批
	approvals, err := ctrl.service.FindMyCreated(c.Request.Context(), currentUserID, offset, pageSize)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 4. 返回响应（简化版，不计算总数）
	ctrl.HandleOK(c, gin.H{
		"results": approvals,
		"count":   len(approvals),
	})
}

// Delete 删除审批
// @Summary 删除审批
// @Description 删除指定的审批
// @Tags approvals
// @Produce json
// @Param id path string true "审批ID"
// @Success 204 "删除成功"
// @Failure 404 {object} core.ErrorResponse "未找到"
// @Router /approvals/{id}/ [delete]
// @Security BearerAuth
func (ctrl *ApprovalController) Delete(c *gin.Context) {
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
