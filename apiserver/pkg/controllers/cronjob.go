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
	cronJob.IsActive = !cronJob.IsActive

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
// 通过POST请求，接收expr字段来校验表达式是否有效，并计算下次执行时间
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
// 根据传递的字段动态更新定时任务，直接使用map[string]interface{}处理
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
