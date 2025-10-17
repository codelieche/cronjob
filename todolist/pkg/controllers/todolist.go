package controllers

import (
	"fmt"
	"time"

	"github.com/codelieche/todolist/pkg/controllers/forms"
	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/middleware"
	"github.com/codelieche/todolist/pkg/utils/controllers"
	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"github.com/codelieche/todolist/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TodoListController 待办事项控制器
type TodoListController struct {
	controllers.BaseController
	service core.TodoListService
}

// parseTimeWithFormats 尝试使用多种格式解析时间字符串（容错处理）
// 支持的格式：
// 1. RFC3339: 2006-01-02T15:04:05Z07:00 (标准格式，带时区)
// 2. RFC3339Nano: 2006-01-02T15:04:05.999999999Z07:00 (带纳秒)
// 3. 无时区格式: 2006-01-02T15:04:05 (假定为本地时区)
// 4. 带毫秒无时区: 2006-01-02T15:04:05.000 (假定为本地时区)
func (ctrl *TodoListController) parseTimeWithFormats(timeStr string) (time.Time, error) {
	// 定义支持的时间格式列表
	formats := []string{
		time.RFC3339,               // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,           // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05Z",     // UTC 格式
		"2006-01-02T15:04:05.000Z", // UTC 带毫秒
		"2006-01-02T15:04:05",      // 无时区
		"2006-01-02T15:04:05.000",  // 无时区带毫秒
		"2006-01-02 15:04:05",      // 空格分隔
	}

	var lastErr error
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			// 如果解析成功且格式不包含时区信息，转换为 UTC
			if t.Location() == time.UTC || format == "2006-01-02T15:04:05Z" || format == "2006-01-02T15:04:05.000Z" {
				return t, nil
			}
			// 对于本地时间，保持原样返回（MySQL 会根据服务器时区处理）
			return t, nil
		} else {
			lastErr = err
		}
	}

	// 所有格式都解析失败
	return time.Time{}, lastErr
}

// TodoDetailResponse 待办事项详情响应（包含父任务和子任务列表）
// 🔥 用于详情接口，返回完整的父任务信息、子任务列表和进度信息
type TodoDetailResponse struct {
	*core.TodoList
	Parent   *core.TodoList   `json:"parent,omitempty"` // 🔥 父任务信息（方便前端显示面包屑和跳转）
	Children []*core.TodoList `json:"children"`         // 子任务列表（不分页，最多100条）
	Progress float64          `json:"progress"`         // 完成进度（0-100）
}

// NewTodoListController 创建待办事项控制器
func NewTodoListController(service core.TodoListService) *TodoListController {
	return &TodoListController{
		service: service,
	}
}

// Create 创建待办事项
// @Summary 创建待办事项
// @Description 创建新的待办事项
// @Tags TodoList
// @Accept json
// @Produce json
// @Param todolist body forms.TodoListCreateForm true "待办事项创建表单"
// @Success 201 {object} types.Response{data=core.TodoList} "创建成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/ [post]
func (ctrl *TodoListController) Create(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 绑定表单数据
	var form forms.TodoListCreateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Error("bind form error", zap.Error(err))
		ctrl.HandleError400(c, err)
		return
	}

	// 解析 ParentID
	var parentID *uuid.UUID
	if form.ParentID != nil && *form.ParentID != "" {
		if parsedParentID, err := uuid.Parse(*form.ParentID); err != nil {
			logger.Error("invalid parent_id format", zap.Error(err), zap.String("parent_id", *form.ParentID))
			ctrl.HandleError400(c, core.ErrBadRequest)
			return
		} else {
			parentID = &parsedParentID

			// 🔥🔥 新增：层级限制验证（禁止创建孙任务）
			// 检查父任务是否也有父任务（即父任务是子任务）
			parentTodo, err := ctrl.service.FindByID(ctx, parsedParentID.String())
			if err != nil {
				logger.Error("find parent todo error", zap.Error(err), zap.String("parent_id", *form.ParentID))
				ctrl.HandleError400(c, fmt.Errorf("父任务不存在或无法访问"))
				return
			}

			// 如果父任务本身也有父任务，说明要创建的是孙任务，拒绝创建
			if parentTodo.ParentID != nil {
				logger.Warn("cannot create grandchild todo",
					zap.String("parent_id", parsedParentID.String()),
					zap.String("grandparent_id", parentTodo.ParentID.String()))
				ctrl.HandleError400(c, fmt.Errorf("❌ 不支持创建孙任务，系统最多支持 2 层任务结构（父任务 → 子任务）"))
				return
			}
		}
	}

	// 🔥 从上下文中获取当前团队ID
	var teamID *uuid.UUID
	if currentTeamID, exists := ctrl.GetCurrentTeamID(c); exists && currentTeamID != "" {
		if parsedTeamID, err := uuid.Parse(currentTeamID); err == nil {
			teamID = &parsedTeamID
			logger.Debug("设置待办事项团队ID", zap.String("team_id", currentTeamID))
		} else {
			logger.Warn("无效的团队ID格式", zap.String("team_id", currentTeamID), zap.Error(err))
		}
	}

	// 创建待办事项对象
	todo := &core.TodoList{
		UserID:      user.UserID,
		TeamID:      teamID,
		Project:     form.Project,
		ParentID:    parentID,
		Title:       form.Title,
		Description: form.Description,
		Priority:    form.Priority,
		Category:    form.Category,
		Tags:        form.Tags,
		StartTime:   form.StartTime, // 🔥 新增：开始时间
		Deadline:    form.Deadline,
		Progress:    form.Progress, // 🔥 新增：手动进度
	}

	// 处理 metadata 字段
	if form.Metadata != nil {
		if err := todo.SetMetadata(form.Metadata); err != nil {
			logger.Error("set metadata error", zap.Error(err))
			ctrl.HandleError400(c, err)
			return
		}
	}

	// 创建待办事项
	result, err := ctrl.service.Create(ctx, todo)
	if err != nil {
		logger.Error("create todo error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	ctrl.HandleCreated(c, result)
}

// List 获取待办事项列表
// @Summary 获取待办事项列表
// @Description 获取当前用户的待办事项列表，支持分页、过滤、搜索和排序。通过view_all_teams参数可以查看跨团队数据：管理员查看所有团队，普通用户查看自己所属的所有团队
// @Tags TodoList
// @Accept json
// @Produce json
// @Param status query string false "状态过滤" Enums(pending,progress,completed,canceled)
// @Param category query string false "分类过滤"
// @Param priority query int false "优先级过滤" minimum(1) maximum(5)
// @Param tags query string false "标签过滤"
// @Param search query string false "搜索关键词"
// @Param page query int false "页码" minimum(1) default(1)
// @Param page_size query int false "每页大小" minimum(1) maximum(100) default(10)
// @Param ordering query string false "排序规则" example("-created_at")
// @Param view_all_teams query boolean false "查看跨团队数据（管理员：所有团队，普通用户：自己所属团队）" example(true)
// @Success 200 {object} types.ResponseList{results=[]core.TodoList} "获取成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/ [get]
func (ctrl *TodoListController) List(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 解析分页参数
	pagination := ctrl.ParsePagination(c)
	offset := (pagination.Page - 1) * pagination.PageSize

	// 定义过滤选项
	filterOptions := []*filters.FilterOption{
		&filters.FilterOption{
			QueryKey: "id",
			Column:   "id",
			Op:       filters.FILTER_EQ,
		},
		// 父任务过滤：parent_id 等于指定值
		&filters.FilterOption{
			QueryKey: "parent_id",
			Column:   "parent_id",
			Op:       filters.FILTER_EQ,
		},
		// 子任务过滤：parent_id_is_null=true 表示查询一级任务（无父任务）
		&filters.FilterOption{
			QueryKey: "parent_id_is_null",
			Column:   "parent_id",
			Op:       filters.FILTER_IS_NULL,
		},
		&filters.FilterOption{
			QueryKey: "project",
			Column:   "project",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "status",
			Column:   "status",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "category",
			Column:   "category",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "priority",
			Column:   "priority",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "tags",
			Column:   "tags",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "tags__contains",
			Column:   "tags",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "deadline__gte",
			Column:   "deadline",
			Op:       filters.FILTER_GTE,
		},
		&filters.FilterOption{
			QueryKey: "deadline__lte",
			Column:   "deadline",
			Op:       filters.FILTER_LTE,
		},
		// 🔥 日历视图优化：支持 start_time 范围查询（用于查询跨月任务）
		&filters.FilterOption{
			QueryKey: "start_time__gte",
			Column:   "start_time",
			Op:       filters.FILTER_GTE,
		},
		&filters.FilterOption{
			QueryKey: "start_time__lte",
			Column:   "start_time",
			Op:       filters.FILTER_LTE,
		},
	}

	// 定义搜索字段
	searchFields := []string{"title", "description"}

	// 定义可排序字段
	orderingFields := []string{"created_at", "updated_at", "title", "priority", "deadline", "status", "parent_id", "project"}
	defaultOrdering := "-created_at"

	// 创建过滤器动作
	filterActions := ctrl.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员跳过）
	filterActions = ctrl.AppendTeamFilterWithOptions(c, filterActions, true)

	// 获取待办事项列表
	todos, err := ctrl.service.GetUserTodos(ctx, user.UserID, offset, pagination.PageSize, filterActions...)
	if err != nil {
		logger.Error("list todos error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	// 获取总数
	total, err := ctrl.service.CountUserTodos(ctx, user.UserID, filterActions...)
	if err != nil {
		logger.Error("count todos error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	// 构建分页响应
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  todos,
	}

	// 返回结果
	ctrl.HandleOK(c, result)
}

// Find 根据ID获取待办事项
// @Summary 根据ID获取待办事项
// @Description 根据ID获取单个待办事项的详细信息（包含父任务信息和子任务列表，方便前端显示面包屑和跳转）
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Success 200 {object} types.Response{data=TodoDetailResponse} "获取成功，返回任务详情、父任务信息（如果存在）、子任务列表和进度"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/ [get]
func (ctrl *TodoListController) Find(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取ID参数
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 根据ID获取待办事项
	todo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("find todo error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 🔥 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, todo.UserID, todo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 🔥🔥 获取父任务信息（如果存在）
	var parent *core.TodoList
	if todo.ParentID != nil {
		parent, err = ctrl.service.FindByID(ctx, todo.ParentID.String())
		if err != nil {
			logger.Warn("get parent todo error",
				zap.Error(err),
				zap.String("parent_id", todo.ParentID.String()),
				zap.String("current_id", id))
			// 即使获取父任务失败，也继续返回子任务信息
			parent = nil
		}
	}

	// 🔥🔥 获取子任务列表（不分页，最多100条）
	children, err := ctrl.service.GetChildTodos(ctx, id)
	if err != nil {
		logger.Error("get child todos error", zap.Error(err), zap.String("parent_id", id))
		// 即使获取子任务失败，也返回父任务信息
		children = []*core.TodoList{}
	}

	// 🔥🔥 构建详情响应（包含父任务、子任务列表和进度）
	response := &TodoDetailResponse{
		TodoList: todo,
		Parent:   parent, // 🔥 父任务信息（方便前端显示和跳转）
		Children: children,
		Progress: todo.GetProgress(),
	}

	ctrl.HandleOK(c, response)
}

// Update 更新待办事项
// @Summary 更新待办事项
// @Description 完整更新待办事项信息
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Param todolist body forms.TodoListUpdateForm true "待办事项更新表单"
// @Success 200 {object} types.Response{data=core.TodoList} "更新成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/ [put]
func (ctrl *TodoListController) Update(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取ID参数
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err))
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 绑定表单数据
	var form forms.TodoListUpdateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Error("bind form error", zap.Error(err))
		ctrl.HandleError400(c, err)
		return
	}

	// 解析 ParentID
	var parentID *uuid.UUID
	if form.ParentID != nil && *form.ParentID != "" {
		if parsedParentID, err := uuid.Parse(*form.ParentID); err != nil {
			logger.Error("invalid parent_id format", zap.Error(err), zap.String("parent_id", *form.ParentID))
			ctrl.HandleError400(c, core.ErrBadRequest)
			return
		} else {
			parentID = &parsedParentID
		}
	}

	// 🔥 获取现有的待办事项以保留TeamID
	existingTodo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("find existing todo error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 🔥 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, existingTodo.UserID, existingTodo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 创建待办事项对象，保留现有的TeamID
	todo := &core.TodoList{
		ID:          uuidID,
		UserID:      user.UserID,
		TeamID:      existingTodo.TeamID, // 保留现有的团队ID
		Project:     form.Project,
		ParentID:    parentID,
		Title:       form.Title,
		Description: form.Description,
		Status:      form.Status,
		Priority:    form.Priority,
		Category:    form.Category,
		Tags:        form.Tags,
		StartTime:   form.StartTime, // 🔥 新增：开始时间
		Deadline:    form.Deadline,
		Progress:    form.Progress, // 🔥 新增：手动进度
	}

	// 处理 metadata 字段
	if form.Metadata != nil {
		if err := todo.SetMetadata(form.Metadata); err != nil {
			logger.Error("set metadata error", zap.Error(err))
			ctrl.HandleError400(c, err)
			return
		}
	}

	// 更新待办事项
	result, err := ctrl.service.Update(ctx, todo)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("update todo error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	ctrl.HandleOK(c, result)
}

// Delete 删除待办事项
// @Summary 删除待办事项
// @Description 根据ID删除待办事项
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Success 204 "删除成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/ [delete]
func (ctrl *TodoListController) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取ID参数
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 🔥 先获取待办事项以验证权限
	todo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("find todo for delete error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 🔥 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, todo.UserID, todo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 删除待办事项
	err = ctrl.service.DeleteByID(ctx, id)
	if err != nil {
		logger.Error("delete todo error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	ctrl.HandleNoContent(c)
}

// Patch 部分更新待办事项
// @Summary 部分更新待办事项
// @Description 部分更新待办事项的某些字段，支持传递任意字段的map格式
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Param updates body object true "要更新的字段" example({"title": "新标题", "status": "completed"})
// @Success 200 {object} types.Response{data=core.TodoList} "更新成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/ [patch]
func (ctrl *TodoListController) Patch(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 1. 获取待办事项的id
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 2. 检查待办事项是否存在并验证权限
	todo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			ctrl.HandleError400(c, err)
		}
		return
	}

	// 🔥 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, todo.UserID, todo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 3. 从请求中获取要更新的字段和值
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ctrl.HandleError400(c, err)
		return
	}

	// 4. 调用服务进行Patch更新
	err = ctrl.service.Patch(ctx, id, updates)
	if err != nil {
		ctrl.HandleError400(c, err)
		return
	}

	// 5. 获取更新后的待办事项信息
	updatedTodo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		ctrl.HandleError400(c, err)
		return
	}

	// 6. 返回成功响应
	ctrl.HandleOK(c, updatedTodo)
}

// UpdateStatus 更新待办事项状态
// @Summary 更新待办事项状态
// @Description 快速更新待办事项的状态
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Param status body forms.TodoListStatusUpdateForm true "状态更新表单"
// @Success 200 {object} types.Response{data=core.TodoList} "更新成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/status/ [put]
func (ctrl *TodoListController) UpdateStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取ID参数
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 🔥 先获取待办事项以验证权限
	todo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("find todo for status update error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 🔥 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, todo.UserID, todo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 绑定表单数据
	var form forms.TodoListStatusUpdateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		logger.Error("bind form error", zap.Error(err))
		ctrl.HandleError400(c, err)
		return
	}

	// 根据状态调用相应的方法
	switch form.Status {
	case core.TodoStatusDone:
		err = ctrl.service.MarkDone(ctx, id)
	case core.TodoStatusRunning:
		err = ctrl.service.MarkRunning(ctx, id)
	case core.TodoStatusPending:
		err = ctrl.service.MarkPending(ctx, id)
	case core.TodoStatusCanceled:
		err = ctrl.service.MarkCanceled(ctx, id)
	default:
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("update todo status error", zap.Error(err))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 获取更新后的待办事项
	updatedTodo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		logger.Error("find updated todo error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	ctrl.HandleOK(c, updatedTodo)
}

// GetStats 获取待办事项统计信息
// @Summary 获取待办事项统计信息
// @Description 获取当前用户的待办事项统计信息，包括状态、优先级、时效性等多维度统计
// @Tags TodoList
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=map[string]interface{}} "获取成功"
// @Failure 401 {object} types.Response "未授权"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/stats/ [get]
func (ctrl *TodoListController) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 统计各状态的数量
	stats := make(map[string]interface{})

	// 🔥 构建基础过滤器（用户ID + 团队过滤）
	userFilter := &filters.FilterOption{
		Column: "user_id",
		Value:  user.UserID,
		Op:     filters.FILTER_EQ,
	}
	baseFilters := []filters.Filter{userFilter}
	baseFilters = ctrl.AppendTeamFilter(c, baseFilters)

	// 总数
	total, err := ctrl.service.Count(ctx, baseFilters...)
	if err != nil {
		logger.Error("count total todos error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}
	stats["total"] = total

	// ========== 各状态统计 ==========
	statusStats := make(map[string]int64)
	statuses := []string{core.TodoStatusPending, core.TodoStatusRunning, core.TodoStatusDone, core.TodoStatusCanceled}
	for _, status := range statuses {
		statusFilter := &filters.FilterOption{
			Column: "status",
			Value:  status,
			Op:     filters.FILTER_EQ,
		}
		statusFilters := append(baseFilters, statusFilter)

		count, err := ctrl.service.Count(ctx, statusFilters...)
		if err != nil {
			logger.Error("count todos by status error", zap.Error(err), zap.String("status", status))
			ctrl.HandleError500(c, err)
			return
		}
		statusStats[status] = count
		stats[status] = count // 保持向后兼容
	}
	stats["status_stats"] = statusStats

	// ========== 优先级统计 ==========
	priorityStats := make(map[string]int64)
	priorities := []int{1, 2, 3, 4, 5}
	for _, priority := range priorities {
		priorityFilter := &filters.FilterOption{
			Column: "priority",
			Value:  priority,
			Op:     filters.FILTER_EQ,
		}
		priorityFilters := append(baseFilters, priorityFilter)

		count, err := ctrl.service.Count(ctx, priorityFilters...)
		if err != nil {
			logger.Error("count todos by priority error", zap.Error(err), zap.Int("priority", priority))
			ctrl.HandleError500(c, err)
			return
		}
		priorityStats[fmt.Sprintf("priority_%d", priority)] = count
	}
	stats["priority_stats"] = priorityStats

	// ========== 时效性统计 ==========
	// 仅统计未完成的（pending + running）
	notDoneFilter := &filters.FilterOption{
		Column: "status",
		Value:  []string{core.TodoStatusPending, core.TodoStatusRunning},
		Op:     filters.FILTER_IN,
	}
	notDoneFilters := append(baseFilters, notDoneFilter)

	timelinessStats := make(map[string]int64)

	// 今日待办（deadline 为今天）
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)

	todayFilter := &filters.FilterOption{
		Column: "deadline",
		Value:  today,
		Op:     filters.FILTER_GTE,
	}
	tomorrowFilter := &filters.FilterOption{
		Column: "deadline",
		Value:  tomorrow,
		Op:     filters.FILTER_LT,
	}
	todayFilters := append(notDoneFilters, todayFilter, tomorrowFilter)
	todayCount, _ := ctrl.service.Count(ctx, todayFilters...)
	timelinessStats["today"] = todayCount

	// 本周待办（deadline 在本周内）
	weekEnd := today.Add(7 * 24 * time.Hour)
	weekFilter := &filters.FilterOption{
		Column: "deadline",
		Value:  weekEnd,
		Op:     filters.FILTER_LT,
	}
	thisWeekFilters := append(notDoneFilters, todayFilter, weekFilter)
	thisWeekCount, _ := ctrl.service.Count(ctx, thisWeekFilters...)
	timelinessStats["this_week"] = thisWeekCount

	// 已过期（deadline < now）
	overdueFilter := &filters.FilterOption{
		Column: "deadline",
		Value:  now,
		Op:     filters.FILTER_LT,
	}
	overdueFilters := append(notDoneFilters, overdueFilter)
	overdueCount, _ := ctrl.service.Count(ctx, overdueFilters...)
	timelinessStats["overdue"] = overdueCount

	// 即将到期（24小时内，now < deadline < tomorrow）
	upcomingStartFilter := &filters.FilterOption{
		Column: "deadline",
		Value:  now,
		Op:     filters.FILTER_GT,
	}
	upcomingFilters := append(notDoneFilters, upcomingStartFilter, tomorrowFilter)
	upcomingCount, _ := ctrl.service.Count(ctx, upcomingFilters...)
	timelinessStats["upcoming"] = upcomingCount

	stats["timeliness_stats"] = timelinessStats

	// ========== 完成率统计 ==========
	completionStats := make(map[string]interface{})
	activeTotal := statusStats[core.TodoStatusPending] + statusStats[core.TodoStatusRunning] + statusStats[core.TodoStatusDone]
	if activeTotal > 0 {
		completionRate := float64(statusStats[core.TodoStatusDone]) / float64(activeTotal) * 100
		completionStats["rate"] = fmt.Sprintf("%.1f", completionRate)
		completionStats["done_count"] = statusStats[core.TodoStatusDone]
		completionStats["total_count"] = activeTotal
	} else {
		completionStats["rate"] = "0.0"
		completionStats["done_count"] = 0
		completionStats["total_count"] = 0
	}
	stats["completion_stats"] = completionStats

	ctrl.HandleOK(c, stats)
}

// MarkDoneWithChildren 标记任务及其所有子任务为已完成（批量操作）
// @Summary 批量完成任务及其子任务
// @Description 将指定的任务及其所有子任务标记为已完成（使用事务保证原子性）
// @Tags TodoList
// @Accept json
// @Produce json
// @Param id path string true "待办事项ID" format(uuid)
// @Success 200 {object} types.Response{data=string} "操作成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/{id}/complete-with-children/ [put]
func (ctrl *TodoListController) MarkDoneWithChildren(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 获取ID参数
	id := c.Param("id")
	if id == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 🔥 验证待办事项是否存在并检查权限
	todo, err := ctrl.service.FindByID(ctx, id)
	if err != nil {
		if err == core.ErrNotFound {
			ctrl.Handle404(c, err)
		} else {
			logger.Error("find todo error", zap.Error(err), zap.String("id", id))
			ctrl.HandleError500(c, err)
		}
		return
	}

	// 验证用户权限
	if err := ctrl.ValidateResourceAccess(c, todo.UserID, todo.TeamID); err != nil {
		ctrl.Handle403(c, err)
		return
	}

	// 🔥🔥 批量完成任务及其所有子任务（使用事务）
	if err := ctrl.service.MarkDoneWithChildren(ctx, id); err != nil {
		logger.Error("mark done with children error", zap.Error(err), zap.String("id", id))
		ctrl.HandleError500(c, err)
		return
	}

	logger.Info("mark done with children success",
		zap.String("id", id),
		zap.String("title", todo.Title),
		zap.Int("children_count", todo.ChildrenCount))

	ctrl.HandleOK(c, gin.H{
		"message": fmt.Sprintf("✅ 任务「%s」及其 %d 个子任务已全部完成", todo.Title, todo.ChildrenCount),
		"id":      id,
	})
}

// GetByTimeRange 获取时间区间内的待办事项（日历视图专用）
// @Summary 获取时间区间内的待办事项
// @Description 获取指定时间区间内的待办事项，使用 OR 逻辑：start_time 在区间内 OR deadline 在区间内 OR 跨区间任务。专为日历视图设计。支持 parent_id_is_null=true 只查询父任务（不含子任务）。
// @Tags TodoList
// @Accept json
// @Produce json
// @Param start_time query string true "区间开始时间" format(date-time) example(2024-10-01T00:00:00Z)
// @Param end_time query string true "区间结束时间" format(date-time) example(2024-10-31T23:59:59Z)
// @Param status query string false "状态过滤" Enums(pending,running,done,canceled)
// @Param parent_id_is_null query boolean false "只查询父任务（不含子任务）" example(true)
// @Param page query int false "页码" minimum(1) default(1)
// @Param page_size query int false "每页大小" minimum(1) maximum(500) default(100)
// @Success 200 {object} types.ResponseList{results=[]core.TodoList} "获取成功"
// @Failure 400 {object} types.Response "参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 500 {object} types.Response "内部错误"
// @Security BearerAuth
// @Security TeamAuth
// @Router /todolist/calendar/ [get]
func (ctrl *TodoListController) GetByTimeRange(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取当前用户信息
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		ctrl.Handle401(c, core.ErrUnauthorized)
		return
	}

	// 解析时间参数
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	if startTimeStr == "" || endTimeStr == "" {
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 🔥 支持多种时间格式解析（容错处理）
	startTime, err := ctrl.parseTimeWithFormats(startTimeStr)
	if err != nil {
		logger.Error("parse start_time error", zap.Error(err), zap.String("start_time", startTimeStr))
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	endTime, err := ctrl.parseTimeWithFormats(endTimeStr)
	if err != nil {
		logger.Error("parse end_time error", zap.Error(err), zap.String("end_time", endTimeStr))
		ctrl.HandleError400(c, core.ErrBadRequest)
		return
	}

	// 解析分页参数
	pagination := ctrl.ParsePagination(c)
	offset := (pagination.Page - 1) * pagination.PageSize

	// 🔥 构建其他过滤器（状态、团队、父任务等）
	filterOptions := []*filters.FilterOption{
		&filters.FilterOption{
			QueryKey: "status",
			Column:   "status",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "priority",
			Column:   "priority",
			Op:       filters.FILTER_EQ,
		},
		// 🔥 日历视图：只显示父任务（parent_id_is_null=true）
		&filters.FilterOption{
			QueryKey: "parent_id_is_null",
			Column:   "parent_id",
			Op:       filters.FILTER_IS_NULL,
		},
	}

	searchFields := []string{}
	orderingFields := []string{}    // 🔥 禁止自定义排序（日历视图使用 Store 层的固定排序）
	defaultOrdering := "start_time" // 🔥 空字符串（不添加额外排序）

	filterActions := ctrl.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 添加团队过滤器
	filterActions = ctrl.AppendTeamFilterWithOptions(c, filterActions, true)

	// 🔥🔥 调用Service层的时间区间查询方法
	todos, err := ctrl.service.GetTodosByTimeRange(ctx, user.UserID, startTime, endTime, offset, pagination.PageSize, filterActions...)
	if err != nil {
		logger.Error("get todos by time range error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	// 获取总数
	total, err := ctrl.service.CountTodosByTimeRange(ctx, user.UserID, startTime, endTime, filterActions...)
	if err != nil {
		logger.Error("count todos by time range error", zap.Error(err))
		ctrl.HandleError500(c, err)
		return
	}

	// 构建分页响应
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  todos,
	}

	// 返回结果
	ctrl.HandleOK(c, result)
}
