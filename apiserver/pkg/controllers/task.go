package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
)

// TaskController 任务控制器
type TaskController struct {
	controllers.BaseController
	service          core.TaskService
	dispatchService  core.DispatchService  // 用于手动重试
	websocketService core.WebsocketService // 用于发送stop/kill指令
}

// NewTaskController 创建TaskController实例
func NewTaskController(service core.TaskService, dispatchService core.DispatchService, websocketService core.WebsocketService) *TaskController {
	return &TaskController{
		service:          service,
		dispatchService:  dispatchService,
		websocketService: websocketService,
	}
}

// Create 创建任务
// @Summary 创建任务
// @Description 创建新的任务执行记录
// @Tags tasks
// @Accept json
// @Produce json
// @Param task body forms.TaskCreateForm true "任务创建表单"
// @Success 201 {object} core.Task "创建成功的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 409 {object} core.ErrorResponse "任务已存在"
// @Router /task/ [post]
// @Security BearerAuth
func (controller *TaskController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.TaskCreateForm
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
	task := form.ToTask()

	// 4. 调用服务创建任务
	createdTask, err := controller.service.Create(c.Request.Context(), task)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdTask)
}

// Find 获取任务信息
// @Summary 根据ID获取任务
// @Description 根据任务ID获取任务执行记录详细信息
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} core.Task "任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/ [get]
// @Security BearerAuth
func (controller *TaskController) Find(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 调用服务获取任务
	task, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回任务信息
	controller.HandleOK(c, task)
}

// Update 更新任务信息
// @Summary 更新任务
// @Description 根据ID更新任务的完整信息
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param task body forms.TaskInfoForm true "任务更新表单"
// @Success 200 {object} core.Task "更新后的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/ [put]
// @Security BearerAuth
func (controller *TaskController) Update(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 获取任务信息
	task, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单数据
	var form forms.TaskInfoForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 对表单进行校验
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 更新任务信息
	if form.Project != "" {
		task.Project = form.Project
	}

	if form.Category != "" {
		task.Category = form.Category
	}

	if form.Name != "" {
		task.Name = form.Name
	}

	// 更新IsGroup字段
	task.IsGroup = &form.IsGroup

	// 更新TaskOrder字段
	task.TaskOrder = form.TaskOrder

	// 更新Timeout字段
	task.Timeout = form.Timeout

	if form.Command != "" {
		task.Command = form.Command
	}

	if form.Args != "" {
		task.Args = form.Args
	}

	if form.Description != "" {
		task.Description = form.Description
	}

	if !form.TimePlan.IsZero() {
		task.TimePlan = form.TimePlan
	}

	if !form.TimeoutAt.IsZero() {
		task.TimeoutAt = form.TimeoutAt
	}

	if form.Status != "" {
		task.Status = form.Status
	}

	if form.Output != "" {
		task.Output = form.Output
	}

	task.SaveLog = &form.SaveLog
	task.RetryCount = form.RetryCount
	task.MaxRetry = form.MaxRetry
	task.IsStandalone = &form.IsStandalone

	if form.WorkerName != "" {
		task.WorkerName = form.WorkerName
	}

	// 处理CronJob（指针类型）
	if form.CronJob != "" {
		if parsedID, err := uuid.Parse(form.CronJob); err == nil {
			task.CronJob = &parsedID
		}
	} else {
		task.CronJob = nil
	}

	// 处理Previous（指针类型）
	if form.Previous != "" {
		if parsedID, err := uuid.Parse(form.Previous); err == nil {
			task.Previous = &parsedID
		}
	} else {
		task.Previous = nil
	}

	// 处理Next（指针类型）
	if form.Next != "" {
		if parsedID, err := uuid.Parse(form.Next); err == nil {
			task.Next = &parsedID
		}
	} else {
		task.Next = nil
	}

	// 处理WorkerID（指针类型）
	if form.WorkerID != "" {
		if parsedID, err := uuid.Parse(form.WorkerID); err == nil {
			task.WorkerID = &parsedID
		}
	} else {
		task.WorkerID = nil
	}

	// 6. 调用服务更新任务
	updatedTask, err := controller.service.Update(c.Request.Context(), task)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 7. 返回成功响应
	controller.HandleOK(c, updatedTask)
}

// Delete 删除任务
// @Summary 删除任务
// @Description 根据ID删除指定的任务记录
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]string "删除成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/ [delete]
// @Security BearerAuth
func (controller *TaskController) Delete(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 调用服务删除任务
	err := controller.service.DeleteByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回成功响应
	controller.HandleOK(c, map[string]string{"message": "任务删除成功"})
}

// List 获取任务列表
// @Summary 获取任务列表
// @Description 获取任务执行记录列表，支持分页和过滤。如果提供X-TEAM-ID，则只返回该团队的任务。通过view_all_teams参数可以查看跨团队数据：管理员查看所有团队，普通用户查看自己所属的所有团队
// @Tags tasks
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param project query string false "项目名称过滤"
// @Param category query string false "分类过滤"
// @Param name query string false "任务名称过滤"
// @Param status query string false "任务状态过滤"
// @Param cronjob query string false "定时任务ID过滤"
// @Param search query string false "搜索关键词"
// @Param view_all_teams query boolean false "查看跨团队数据（管理员：所有团队，普通用户：自己所属团队）" example(true)
// @Success 200 {object} types.ResponseList "分页的任务列表"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "团队不存在"
// @Router /task/ [get]
// @Security BearerAuth
// @Security TeamAuth
func (controller *TaskController) List(c *gin.Context) {
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
			QueryKey: "project",
			Column:   "project",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "category",
			Column:   "category",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "cronjob",
			Column:   "cronjob",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "status",
			Column:   "status",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "worker_id",
			Column:   "worker_id",
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
			QueryKey: "is_group",
			Column:   "is_group",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "task_order",
			Column:   "task_order",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "timeout",
			Column:   "timeout",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "is_standalone",
			Column:   "is_standalone",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "start_time",
			Column:   "created_at",
			Op:       filters.FILTER_GTE,
		},
		&filters.FilterOption{
			QueryKey: "end_time",
			Column:   "created_at",
			Op:       filters.FILTER_LTE,
		},
		&filters.FilterOption{
			QueryKey: "team_id",
			Column:   "team_id",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description", "command"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "time_plan", "time_start", "time_end", "name", "status", "task_order"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 🔥 添加团队过滤器（支持管理员查看所有团队数据）
	filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, true)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取任务列表
	tasks, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
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

	// 9. 构建分页结果
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    count,
		Results:  tasks,
	}

	// 10. 返回结果
	controller.HandleOK(c, result)
}

// UpdateStatus 更新任务状态
// @Summary 更新任务执行状态
// @Description 更新指定任务的执行状态
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param status body object{status=string} true "任务状态" example({"status": "running"})
// @Success 200 {object} core.Task "更新后的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/update-status/ [put]
// @Security BearerAuth
func (controller *TaskController) UpdateStatus(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 获取新的状态
	status := c.Query("status")
	if status == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 3. 调用服务更新状态
	err := controller.service.UpdateStatus(c.Request.Context(), id, status)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 4. 返回成功响应
	controller.HandleOK(c, gin.H{"success": true, "message": "任务状态更新成功"})
}

// UpdateOutput 更新任务输出
// @Summary 更新任务执行输出
// @Description 更新指定任务的执行输出结果
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param output body object{output=string} true "任务输出" example({"output": "执行成功"})
// @Success 200 {object} core.Task "更新后的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/update-output/ [put]
// @Security BearerAuth
func (controller *TaskController) UpdateOutput(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 获取新的输出
	var data struct {
		Output string `json:"output" binding:"required"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 调用服务更新输出
	err := controller.service.UpdateOutput(c.Request.Context(), id, data.Output)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 4. 返回成功响应
	controller.HandleOK(c, gin.H{"success": true, "message": "任务输出更新成功"})
}

// Patch 动态更新任务信息
// @Summary 部分更新任务
// @Description 根据传递的字段动态更新任务的部分信息
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param updates body map[string]interface{} true "要更新的字段和值" example({"status": "completed", "output": "执行完成"})
// @Success 200 {object} core.Task "更新后的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Router /task/{id}/ [patch]
// @Security BearerAuth
func (controller *TaskController) Patch(c *gin.Context) {
	// 1. 获取任务的id
	id := c.Param("id")

	// 2. 检查任务是否存在
	_, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}
	// 检查传递的数据是否合规: 由于需要2次绑定，所以使用了c.ShouldBindBodyWith
	var form forms.TaskInfoForm
	if err := c.ShouldBindBodyWith(&form, binding.JSON); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	} else {
		// 校验一下交单即可，其实我们不用它，我们直接使用后续的updates再取一次数据
		// 但是我们这里校验一下，因为我们后续的updates是直接使用的，我们不希望用户传递一些不可更新的字段
		if err := form.Validate(); err != nil {
			controller.HandleError(c, err, http.StatusBadRequest)
			return
		}
	}

	// 3. 从请求中获取要更新的字段和值
	var updates map[string]interface{}
	if err := c.ShouldBindBodyWith(&updates, binding.JSON); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 调用服务进行Patch更新
	err = controller.service.Patch(c.Request.Context(), id, updates)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 获取更新后的任务信息
	updatedTask, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	controller.HandleOK(c, updatedTask)
}

// Retry 手动重试任务
// @Summary 手动重试失败的任务
// @Description 立即创建一个新的重试任务（pending状态），用于手动触发失败任务的重试
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} core.Task "创建的重试任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误或任务不满足重试条件"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Failure 500 {object} core.ErrorResponse "服务器错误"
// @Router /task/{id}/retry/ [post]
// @Security BearerAuth
func (controller *TaskController) Retry(c *gin.Context) {
	// 1. 获取任务ID
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 验证任务ID格式
	if _, err := uuid.Parse(id); err != nil {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 3. 调用dispatchService重试任务
	retryTask, err := controller.dispatchService.RetryTask(c.Request.Context(), id)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 返回成功响应
	controller.HandleOK(c, retryTask)
}

// Cancel 取消待执行任务
// @Summary 取消待执行的任务
// @Description 取消pending状态的任务，使用分布式锁确保并发安全
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} core.Task "取消后的任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误或任务状态不允许取消"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Failure 500 {object} core.ErrorResponse "服务器错误"
// @Router /task/{id}/cancel/ [post]
// @Security BearerAuth
func (controller *TaskController) Cancel(c *gin.Context) {
	// 1. 获取任务ID
	id := c.Param("id")
	if id == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 验证任务ID格式
	if _, err := uuid.Parse(id); err != nil {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 3. 调用service取消任务
	canceledTask, err := controller.service.Cancel(c.Request.Context(), id)
	if err != nil {
		// 根据错误类型返回不同的状态码
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 4. 返回成功响应
	controller.HandleOK(c, canceledTask)
}

// StopTask 停止任务（支持优雅停止和强制终止）
// @Summary 停止任务
// @Description 停止正在运行的任务，通过force参数控制停止方式
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param force query bool false "是否强制终止 (false=SIGTERM优雅停止, true=SIGKILL强制终止)"
// @Param body body forms.StopTaskRequest false "请求体参数（可选，与query参数二选一）"
// @Success 200 {object} map[string]interface{} "停止指令已发送"
// @Failure 400 {object} core.ErrorResponse "任务状态不是running或worker_id为空"
// @Failure 404 {object} core.ErrorResponse "任务不存在"
// @Failure 503 {object} core.ErrorResponse "Worker离线，无法发送指令"
// @Router /task/{id}/stop [post]
// @Security BearerAuth
func (controller *TaskController) StopTask(c *gin.Context) {
	// 1. 获取任务ID
	taskID := c.Param("id")
	if taskID == "" {
		controller.HandleError(c, core.ErrBadRequest, http.StatusBadRequest)
		return
	}

	// 2. 解析force参数（支持query参数和body参数）
	var req forms.StopTaskRequest

	// 优先读取query参数
	if forceStr := c.Query("force"); forceStr != "" {
		req.Force = forceStr == "true" || forceStr == "1"
	} else {
		// 尝试从body读取（忽略错误，默认force=false）
		_ = c.ShouldBindJSON(&req)
	}

	// 3. 查询任务
	task, err := controller.service.FindByID(c.Request.Context(), taskID)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 4. 验证任务状态（只有running状态的任务可以停止）
	if task.Status != core.TaskStatusRunning {
		err := fmt.Errorf("任务状态不是running，当前状态: %s", task.Status)
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 验证worker_id（必须有值）
	if task.WorkerID == nil || task.WorkerID.String() == "" {
		err := fmt.Errorf("任务的worker_id为空，无法发送停止指令")
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 根据force参数决定action类型
	action := core.TaskActionStop
	actionText := "停止"
	if req.Force {
		action = core.TaskActionKill
		actionText = "强制终止"
	}

	// 7. 直接调用WebSocket服务发送指令
	workerID := task.WorkerID.String()
	if err := controller.websocketService.SendTaskAction(workerID, action, task); err != nil {
		// 发送失败（Worker离线或其他错误）
		errMsg := fmt.Errorf("发送%s指令失败: %s", actionText, err.Error())
		controller.HandleError(c, errMsg, http.StatusServiceUnavailable)
		return
	}

	// 8. 返回成功
	controller.HandleOK(c, map[string]interface{}{
		"message":   "任务" + actionText + "指令已发送",
		"task_id":   task.ID.String(),
		"worker_id": workerID,
		"action":    string(action),
		"force":     req.Force,
		"sent_at":   time.Now().Format("2006-01-02 15:04:05"),
	})
}
