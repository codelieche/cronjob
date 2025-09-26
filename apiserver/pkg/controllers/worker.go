package controllers

import (
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
)

// WorkerController 工作节点控制器
type WorkerController struct {
	controllers.BaseController
	service core.WorkerService
}

// NewWorkerController 创建WorkerController实例
func NewWorkerController(service core.WorkerService) *WorkerController {
	return &WorkerController{
		service: service,
	}
}

// Create 创建工作节点
// @Summary 注册工作节点
// @Description 注册新的Worker工作节点
// @Tags workers
// @Accept json
// @Produce json
// @Param worker body forms.WorkerCreateForm true "工作节点创建表单"
// @Success 201 {object} core.Worker "创建成功的工作节点信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 409 {object} core.ErrorResponse "工作节点已存在"
// @Router /worker/ [post]
// @Security BearerAuth
func (controller *WorkerController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.WorkerCreateForm
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
	worker := form.ToWorker()

	// 4. 调用服务创建工作节点
	createdWorker, err := controller.service.Create(c.Request.Context(), worker)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdWorker)
}

// Find 获取工作节点信息
// @Summary 根据ID获取工作节点
// @Description 根据工作节点ID获取详细信息
// @Tags workers
// @Accept json
// @Produce json
// @Param id path string true "工作节点ID"
// @Success 200 {object} core.Worker "工作节点信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "工作节点不存在"
// @Router /worker/{id}/ [get]
// @Security BearerAuth
func (controller *WorkerController) Find(c *gin.Context) {
	// 1. 获取工作节点的id
	id := c.Param("id")

	// 2. 调用服务获取工作节点
	worker, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回工作节点信息
	controller.HandleOK(c, worker)
}

// Update 更新工作节点信息
// @Summary 更新工作节点
// @Description 根据ID更新工作节点的详细信息
// @Tags workers
// @Accept json
// @Produce json
// @Param id path string true "工作节点ID"
// @Param worker body forms.WorkerInfoForm true "工作节点更新表单"
// @Success 200 {object} core.Worker "更新后的工作节点信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "工作节点不存在"
// @Failure 409 {object} core.ErrorResponse "工作节点名称冲突"
// @Router /worker/{id}/ [put]
// @Security BearerAuth
func (controller *WorkerController) Update(c *gin.Context) {
	// 1. 获取工作节点的id
	id := c.Param("id")

	// 2. 获取工作节点信息
	worker, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单数据
	var form forms.WorkerInfoForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 更新工作节点信息
	form.UpdateWorker(worker)

	// 6. 调用服务更新工作节点
	updatedWorker, err := controller.service.Update(c.Request.Context(), worker)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 7. 返回更新后的工作节点信息
	controller.HandleOK(c, updatedWorker)
}

// Delete 删除工作节点
// @Summary 删除工作节点
// @Description 根据ID删除指定的工作节点
// @Tags workers
// @Accept json
// @Produce json
// @Param id path string true "工作节点ID"
// @Success 200 {object} map[string]string "删除成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "工作节点不存在"
// @Router /worker/{id}/ [delete]
// @Security BearerAuth
func (controller *WorkerController) Delete(c *gin.Context) {
	// 1. 获取工作节点的id
	id := c.Param("id")

	// 2. 调用服务删除工作节点
	err := controller.service.DeleteByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回删除成功
	controller.HandleOK(c, map[string]string{"message": "工作节点删除成功"})
}

// List 获取工作节点列表
// @Summary 获取工作节点列表
// @Description 获取工作节点列表，支持分页、搜索和过滤
// @Tags workers
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param search query string false "搜索关键词（在name、description中搜索）"
// @Param id query string false "工作节点ID"
// @Param name query string false "工作节点名称"
// @Param name__contains query string false "工作节点名称包含"
// @Param is_active query bool false "是否活跃"
// @Param deleted query bool false "是否已删除"
// @Param ordering query string false "排序字段" Enums(name, created_at, updated_at, last_active, -name, -created_at, -updated_at, -last_active)
// @Success 200 {object} types.ResponseList "工作节点列表和分页信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /worker/ [get]
// @Security BearerAuth
func (controller *WorkerController) List(c *gin.Context) {
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
			QueryKey: "is_active",
			Column:   "is_active",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "deleted",
			Column:   "deleted",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description"}

	// 4. 定义排序字段
	orderingFields := []string{"name", "created_at", "updated_at", "last_active"}
	defaultOrdering := "-last_active"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取工作节点列表
	workers, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取工作节点总数
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
		Results:  workers,
	}

	// 10. 返回结果
	controller.HandleOK(c, result)
}

// Ping 工作节点心跳接口
// @Summary 工作节点心跳
// @Description 更新工作节点的is_active状态和last_active时间
// @Tags workers
// @Accept json
// @Produce json
// @Param id path string true "工作节点ID"
// @Success 200 {object} map[string]interface{} "心跳响应，包含pong消息和工作节点信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "工作节点不存在"
// @Router /worker/{id}/ping/ [put]
// @Security BearerAuth
func (controller *WorkerController) Ping(c *gin.Context) {
	// 1. 获取工作节点的id
	id := c.Param("id")

	// 2. 调用服务获取工作节点
	worker, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 更新工作节点的is_active状态和last_active时间
	// 如果is_active为false，则设置为true
	isActive := true
	if worker.IsActive == nil || !*worker.IsActive {
		worker.IsActive = &isActive
	}
	// 更新last_active时间为当前时间
	now := time.Now()
	worker.LastActive = &now

	// 4. 调用服务更新工作节点
	updatedWorker, err := controller.service.Update(c.Request.Context(), worker)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 返回成功响应
	controller.HandleOK(c, map[string]interface{}{
		"message": "pong",
		"worker":  updatedWorker,
	})
}
