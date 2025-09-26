package controllers

import (
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
)

// CronJobController 定时任务控制器
type CronJobController struct {
	controllers.BaseController
	service core.CronJobService
}

// NewCronJobController 创建CronJobController实例
func NewCronJobController(service core.CronJobService) *CronJobController {
	return &CronJobController{
		service: service,
	}
}

// Create 创建定时任务
// @Summary 创建定时任务
// @Description 创建新的定时任务，支持cron表达式调度
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param cronjob body forms.CronJobCreateForm true "定时任务创建表单"
// @Success 201 {object} core.CronJob "创建成功的定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 409 {object} core.ErrorResponse "定时任务已存在"
// @Router /cronjob/ [post]
// @Security BearerAuth
func (controller *CronJobController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.CronJobCreateForm
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
	CronJob := form.ToCronJob()

	// 4. 调用服务创建定时任务
	createdCronJob, err := controller.service.Create(c.Request.Context(), CronJob)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdCronJob)
}

// Find 获取定时任务信息
// @Summary 根据ID获取定时任务
// @Description 根据定时任务ID获取详细信息
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param id path string true "定时任务ID"
// @Success 200 {object} core.CronJob "定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Router /cronjob/{id}/ [get]
// @Security BearerAuth
func (controller *CronJobController) Find(c *gin.Context) {
	// 1. 获取定时任务的id
	id := c.Param("id")

	// 2. 调用服务获取定时任务
	cronJob, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回定时任务信息
	controller.HandleOK(c, cronJob)
}

// Update 更新定时任务信息
// @Summary 更新定时任务
// @Description 根据ID更新定时任务的完整信息
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param id path string true "定时任务ID"
// @Param cronjob body forms.CronJobInfoForm true "定时任务更新表单"
// @Success 200 {object} core.CronJob "更新后的定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Failure 409 {object} core.ErrorResponse "定时任务名称冲突"
// @Router /cronjob/{id}/ [put]
// @Security BearerAuth
func (controller *CronJobController) Update(c *gin.Context) {
	// 1. 获取定时任务的id
	id := c.Param("id")

	// 2. 获取定时任务信息
	cronJob, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单数据
	var form forms.CronJobInfoForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 更新定时任务信息
	form.UpdateCronJob(cronJob)

	// 6. 调用服务更新定时任务
	updatedCronJob, err := controller.service.Update(c.Request.Context(), cronJob)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 7. 返回更新后的定时任务信息
	controller.HandleOK(c, updatedCronJob)
}

// Delete 删除定时任务
// @Summary 删除定时任务
// @Description 根据ID删除指定的定时任务
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param id path string true "定时任务ID"
// @Success 200 {object} map[string]string "删除成功信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Router /cronjob/{id}/ [delete]
// @Security BearerAuth
func (controller *CronJobController) Delete(c *gin.Context) {
	// 1. 获取定时任务的id
	id := c.Param("id")

	// 2. 调用服务删除定时任务
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
	controller.HandleOK(c, map[string]string{"message": "定时任务删除成功"})
}

// List 获取定时任务列表
// @Summary 获取定时任务列表
// @Description 获取定时任务列表，支持分页和过滤
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param project query string false "项目名称过滤"
// @Param category query string false "分类过滤"
// @Param name query string false "任务名称过滤"
// @Param is_active query bool false "激活状态过滤"
// @Param search query string false "搜索关键词"
// @Success 200 {object} types.ResponseList "分页的定时任务列表"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /cronjob/ [get]
// @Security BearerAuth
func (controller *CronJobController) List(c *gin.Context) {
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
		&filters.FilterOption{
			QueryKey: "timeout",
			Column:   "timeout",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description", "command"}

	// 4. 定义排序字段
	orderingFields := []string{"name", "created_at", "updated_at", "last_dispatch", "is_active"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取定时任务列表
	cronJobs, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取定时任务总数
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
		Results:  cronJobs,
	}

	// 10. 返回结果
	controller.HandleOK(c, result)
}

// ToggleActive 切换定时任务的激活状态
// @Summary 切换定时任务激活状态
// @Description 切换指定定时任务的激活/停用状态
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param id path string true "定时任务ID"
// @Success 200 {object} core.CronJob "更新后的定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Router /cronjob/{id}/toggle-active/ [put]
// @Security BearerAuth
func (controller *CronJobController) ToggleActive(c *gin.Context) {
	// 1. 获取定时任务的id
	id := c.Param("id")

	// 2. 获取定时任务信息
	cronJob, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 切换激活状态
	newIsActive := cronJob.IsActive == nil || !*cronJob.IsActive
	cronJob.IsActive = &newIsActive

	// 4. 调用服务更新定时任务
	updatedCronJob, err := controller.service.Update(c.Request.Context(), cronJob)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 返回更新后的定时任务信息
	controller.HandleOK(c, updatedCronJob)
}

// FindByProjectAndName 根据项目和名称获取定时任务
// @Summary 根据项目和名称获取定时任务
// @Description 根据项目名称和任务名称获取定时任务信息
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param project path string true "项目名称"
// @Param name path string true "任务名称"
// @Success 200 {object} core.CronJob "定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Router /cronjob/project/{project}/name/{name}/ [get]
// @Security BearerAuth
func (controller *CronJobController) FindByProjectAndName(c *gin.Context) {
	// 1. 获取项目名和任务名
	project := c.Param("project")
	name := c.Param("name")

	// 2. 调用服务获取定时任务
	cronJob, err := controller.service.FindByProjectAndName(c.Request.Context(), project, name)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回定时任务信息
	controller.HandleOK(c, cronJob)
}

// ValidateExpression 验证cron表达式并返回下次执行时间
// @Summary 验证cron表达式
// @Description 验证cron表达式的有效性并计算下次执行时间
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param expression body object{expr=string} true "cron表达式" example({"expr": "0 0 12 * * ?"})
// @Success 200 {object} map[string]interface{} "验证结果和下次执行时间"
// @Failure 400 {object} core.ErrorResponse "表达式无效或请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /cronjob/validate-expression/ [post]
// @Security BearerAuth
func (controller *CronJobController) ValidateExpression(c *gin.Context) {
	// 1. 定义请求参数结构
	req := struct {
		Expr string `json:"expr" binding:"required"`
	}{}

	// 2. 绑定并验证请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 验证cron表达式是否有效
	if !tools.ValidateCronExpression(req.Expr) {
		controller.HandleError(
			c,
			core.ErrBadRequest,
			http.StatusBadRequest,
		)
		return
	}

	// 4. 计算下次执行时间
	nextExecutionTime, err := tools.GetNextExecutionTime(req.Expr, time.Now())
	if err != nil {
		controller.HandleError(
			c,
			core.ErrBadRequest,
			http.StatusBadRequest,
		)
		return
	}

	// 5. 返回成功响应
	controller.HandleOK(c, gin.H{
		"valid":               true,
		"expression":          req.Expr,
		"next_execution_time": nextExecutionTime.Format(time.RFC3339),
	})
}

// Patch 动态更新定时任务信息
// @Summary 部分更新定时任务
// @Description 根据传递的字段动态更新定时任务的部分信息
// @Tags cronjobs
// @Accept json
// @Produce json
// @Param id path string true "定时任务ID"
// @Param updates body map[string]interface{} true "要更新的字段和值" example({"is_active": true, "description": "更新描述"})
// @Success 200 {object} core.CronJob "更新后的定时任务信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Failure 404 {object} core.ErrorResponse "定时任务不存在"
// @Router /cronjob/{id}/ [patch]
// @Security BearerAuth
func (controller *CronJobController) Patch(c *gin.Context) {
	// 1. 获取定时任务的id
	id := c.Param("id")

	// 2. 检查定时任务是否存在
	_, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 从请求中获取要更新的字段和值
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 调用服务进行Patch更新
	err = controller.service.Patch(c.Request.Context(), id, updates)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 获取更新后的定时任务信息
	updatedCronJob, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 6. 返回成功响应
	controller.HandleOK(c, updatedCronJob)
}
