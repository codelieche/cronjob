package controllers

import (
	"fmt"
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TaskLogController 任务日志控制器
type TaskLogController struct {
	controllers.BaseController
	service core.TaskLogService
}

// NewTaskLogController 创建任务日志控制器
func NewTaskLogController(service core.TaskLogService) *TaskLogController {
	return &TaskLogController{
		service: service,
	}
}

// Create 创建任务日志
// @Summary 创建任务日志
// @Description 创建新的任务日志记录
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_log body forms.TaskLogCreateForm true "任务日志创建表单"
// @Success 201 {object} core.TaskLog "创建成功的任务日志信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 409 {object} core.ErrorResponse "任务日志已存在"
// @Router /task-log/ [post]
// @Security BearerAuth
func (controller *TaskLogController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.TaskLogCreateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 对表单进行校验
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 准备创建对象
	taskLog := form.ToTaskLog()

	// 4. 调用服务创建任务日志
	createdTaskLog, err := controller.service.Create(c.Request.Context(), taskLog)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdTaskLog)
}

// Find 根据任务ID获取任务日志信息
// @Summary 根据任务ID获取任务日志
// @Description 根据任务ID获取任务日志信息和内容
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "任务日志信息和内容"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务日志不存在"
// @Router /task-log/{task_id}/ [get]
// @Security BearerAuth
func (controller *TaskLogController) Find(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 调用服务获取任务日志
	taskLog, err := controller.service.FindByTaskID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 获取日志内容
	content, err := controller.service.GetLogContent(c.Request.Context(), taskLog)
	if err != nil {
		// 如果获取内容失败，记录错误但不返回错误，使用空内容
		content = ""
	}

	// 4. 构建响应，包含内容
	response := map[string]interface{}{
		"task_id":    taskLog.TaskID,
		"storage":    taskLog.Storage,
		"path":       taskLog.Path,
		"content":    content,
		"size":       taskLog.Size,
		"created_at": taskLog.CreatedAt,
		"updated_at": taskLog.UpdatedAt,
	}

	controller.HandleOK(c, response)
}

// Update 更新任务日志信息
// @Summary 更新任务日志
// @Description 根据任务ID更新任务日志信息
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param task_log body forms.TaskLogUpdateForm true "任务日志更新表单"
// @Success 200 {object} core.TaskLog "更新后的任务日志信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务日志不存在"
// @Router /task-log/{task_id}/ [put]
// @Security BearerAuth
func (controller *TaskLogController) Update(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 处理表单
	var form forms.TaskLogUpdateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 对表单进行校验
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 获取现有任务日志
	taskLog, err := controller.service.FindByTaskID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 更新任务日志信息
	form.UpdateTaskLog(taskLog)

	// 6. 调用服务更新任务日志
	updatedTaskLog, err := controller.service.Update(c.Request.Context(), taskLog)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 6. 返回成功响应
	controller.HandleOK(c, updatedTaskLog)
}

// Delete 删除任务日志
// @Summary 删除任务日志
// @Description 根据任务ID删除任务日志
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} map[string]string "删除成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务日志不存在"
// @Router /task-log/{task_id}/ [delete]
// @Security BearerAuth
func (controller *TaskLogController) Delete(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 调用服务删除任务日志
	err := controller.service.DeleteByTaskID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回成功响应
	controller.HandleOK(c, gin.H{"message": "删除成功"})
}

// List 获取任务日志列表
// @Summary 获取任务日志列表
// @Description 获取任务日志列表，支持分页、搜索和过滤
// @Tags task-logs
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param search query string false "搜索关键词（在path中搜索）"
// @Param task_id query string false "任务ID"
// @Param storage query string false "存储类型"
// @Param deleted query bool false "是否已删除"
// @Param ordering query string false "排序字段" Enums(created_at, updated_at, size, -created_at, -updated_at, -size)
// @Success 200 {object} types.ResponseList "任务日志列表和分页信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /task-log/ [get]
// @Security BearerAuth
func (controller *TaskLogController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		&filters.FilterOption{
			QueryKey: "task_id",
			Column:   "task_id",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "storage",
			Column:   "storage",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "deleted",
			Column:   "deleted",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"path"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "updated_at", "size"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取任务日志列表
	taskLogs, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取任务日志总数
	total, err := controller.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 9. 为每个日志获取内容
	var results []map[string]interface{}
	for _, taskLog := range taskLogs {
		content, err := controller.service.GetLogContent(c.Request.Context(), taskLog)
		if err != nil {
			// 如果获取内容失败，记录错误但不返回错误，使用空内容
			content = ""
		}

		item := map[string]interface{}{
			"task_id":    taskLog.TaskID,
			"storage":    taskLog.Storage,
			"path":       taskLog.Path,
			"content":    content,
			"size":       taskLog.Size,
			"created_at": taskLog.CreatedAt,
			"updated_at": taskLog.UpdatedAt,
		}
		results = append(results, item)
	}

	// 10. 构建分页结果
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  results,
	}

	// 11. 返回结果
	controller.HandleOK(c, result)
}

// GetContent 获取任务日志内容
// @Summary 获取任务日志内容
// @Description 根据任务ID获取任务日志的具体内容
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} map[string]interface{} "任务日志内容和相关信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务日志不存在"
// @Router /task-log/{task_id}/content/ [get]
// @Security BearerAuth
func (controller *TaskLogController) GetContent(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 获取任务日志
	taskLog, err := controller.service.FindByTaskID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 获取日志内容
	content, err := controller.service.GetLogContent(c.Request.Context(), taskLog)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 返回内容
	response := map[string]interface{}{
		"task_id": taskLog.TaskID,
		"content": content,
		"size":    taskLog.Size,
	}

	controller.HandleOK(c, response)
}

// SaveContent 保存任务日志内容
// @Summary 保存任务日志内容
// @Description 保存或更新任务日志的内容
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param content body object{content=string} true "日志内容" example({"content": "任务执行日志内容"})
// @Success 200 {object} map[string]string "保存成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务日志不存在"
// @Router /task-log/{task_id}/content/ [put]
// @Security BearerAuth
func (controller *TaskLogController) SaveContent(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 获取任务日志
	taskLog, err := controller.service.FindByTaskID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 解析请求体
	var request struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBind(&request); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 保存内容
	err = controller.service.SaveLogContent(c.Request.Context(), taskLog, request.Content)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 返回成功响应
	controller.HandleOK(c, gin.H{"message": "保存成功"})
}

// AppendContent 追加任务日志内容（智能创建+追加）
// @Summary 追加任务日志内容
// @Description 智能追加任务日志内容，如果日志不存在则创建，存在则追加
// @Tags task-logs
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param data body object{task_id=string,storage=string,content=string} true "追加日志数据" example({"task_id": "uuid", "storage": "file", "content": "追加的日志内容"})
// @Success 200 {object} map[string]interface{} "追加成功的任务日志信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /task-log/{task_id}/append/ [post]
// @Security BearerAuth
func (controller *TaskLogController) AppendContent(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("task_id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 解析请求体
	var request struct {
		TaskID  string `json:"task_id"` // 可选，用于验证
		Storage string `json:"storage"` // 可选，用于指定存储类型
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBind(&request); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 验证taskID一致性（如果请求体中提供了task_id）
	if request.TaskID != "" && request.TaskID != taskID {
		controller.HandleError(c, fmt.Errorf("URL中的task_id与请求体中的task_id不一致"), http.StatusBadRequest)
		return
	}

	// 4. 解析UUID
	uuidTaskID, err := uuid.Parse(taskID)
	if err != nil {
		controller.HandleError(c, fmt.Errorf("无效的task_id格式"), http.StatusBadRequest)
		return
	}

	// 5. 准备TaskLog对象
	taskLog := &core.TaskLog{
		TaskID:  uuidTaskID,
		Storage: request.Storage, // 如果为空，Service层会设置默认值
	}

	// 6. 调用智能追加方法（如果不存在则创建，存在则追加）
	taskLog, err = controller.service.AppendLogContent(c.Request.Context(), taskLog, request.Content)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 7. 返回成功响应
	response := map[string]interface{}{
		"task_id": taskLog.TaskID,
		"storage": taskLog.Storage,
		"path":    taskLog.Path,
		"size":    taskLog.Size,
	}
	controller.HandleOK(c, response)
}
