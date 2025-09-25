package controllers

import (
	"net/http"

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
	service core.TaskService
}

// NewTaskController 创建TaskController实例
func NewTaskController(service core.TaskService) *TaskController {
	return &TaskController{
		service: service,
	}
}

// Create 创建任务
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
			Column:   "cron_job",
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
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description", "command"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "time_plan", "time_start", "time_end", "name", "status", "task_order"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

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
// 根据传递的字段动态更新任务，直接使用map[string]interface{}处理
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
