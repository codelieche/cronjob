package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TaskLogController 任务日志控制器
type TaskLogController struct {
	controllers.BaseController
	service     core.TaskLogService
	taskService core.TaskService // 🔥 P2优化：用于自动获取Task的created_at
}

// NewTaskLogController 创建任务日志控制器
func NewTaskLogController(service core.TaskLogService, taskService core.TaskService) *TaskLogController {
	return &TaskLogController{
		service:     service,
		taskService: taskService, // 🔥 注入TaskService
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
// @Param created_at query string false "精确创建时间 (YYYY-MM-DD HH:MM:SS)，用于精确分片定位（性能最优）" example("2025-09-30 12:00:00")
// @Param start_time query string false "开始时间范围 (YYYY-MM-DD)，用于分片查询优化" example("2025-09-01")
// @Param end_time query string false "结束时间范围 (YYYY-MM-DD)，用于分片查询优化" example("2025-09-30")
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

	// 2. 🔥🔥 优雅的优化方式：通过Context传递优化信息
	ctx := controller.parseOptimizationContext(c)

	// 🔥 2.5. P2优化：如果用户没提供时间参数，自动从Task表获取created_at
	// 性能提升：从跨3个月查询（~50ms）降到精确分片查询（~2-5ms，提升10-25倍）
	if ctx == c.Request.Context() { // 说明没有优化信息被添加到context
		if controller.taskService != nil {
			if task, err := controller.taskService.FindByID(c.Request.Context(), taskID); err == nil && task != nil {
				// 成功获取Task，将created_at注入context
				opt := &store.TaskLogOptimization{
					CreatedAt: &task.CreatedAt,
				}
				ctx = store.WithTaskLogOptimization(ctx, opt)
				logger.Debug("自动从Task获取created_at优化TaskLog查询",
					zap.String("task_id", taskID),
					zap.Time("created_at", task.CreatedAt))
			}
		}
	}

	// 3. 🔥🔥 直接使用FindByTaskID，内部已经自动智能优化
	taskLog, err := controller.service.FindByTaskID(ctx, taskID)

	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 🔥 3. 权限控制：验证用户是否有权限访问该TaskLog
	// 需要通过Task表获取team_id来验证权限
	if err := controller.validateTaskLogAccess(c, taskLog.TaskID.String()); err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusForbidden)
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
// @Description 获取任务日志列表，支持分页、搜索和过滤。通过view_all_teams参数可以查看跨团队数据：管理员查看所有团队，普通用户查看自己所属的所有团队。支持时间范围过滤以优化分片查询性能。支持通过cronjob参数过滤特定定时任务的日志。🚀 推荐使用month参数指定月份（格式：202510），性能提升10倍+，只查询指定月份的数据，默认为当前月份，前端可提供"上一月/下一月"切换按钮。
// @Tags task-logs
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param search query string false "搜索关键词（在path中搜索）"
// @Param task_id query string false "任务ID"
// @Param month query string false "🚀 月份（格式：202510，性能最优，默认为当前月份）" example("202510")
// @Param cronjob query string false "定时任务ID（过滤该定时任务产生的所有任务日志）"
// @Param storage query string false "存储类型"
// @Param deleted query bool false "是否已删除"
// @Param start_time query string false "开始时间 (YYYY-MM-DD)" example("2025-09-01")
// @Param end_time query string false "结束时间 (YYYY-MM-DD)" example("2025-09-30")
// @Param ordering query string false "排序字段" Enums(created_at, updated_at, size, -created_at, -updated_at, -size)
// @Param view_all_teams query boolean false "查看跨团队数据（管理员：所有团队，普通用户：自己所属团队）" example(true)
// @Success 200 {object} types.ResponseList "任务日志列表和分页信息"
// @Failure 400 {object} core.ErrorResponse "请求参数错误"
// @Failure 401 {object} core.ErrorResponse "未认证"
// @Router /task-log/ [get]
// @Security BearerAuth
func (controller *TaskLogController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项（🔥 新增时间范围过滤器用于分片优化）
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
		// 🔥 新增时间范围过滤器，用于分片查询优化
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
	searchFields := []string{"path"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "updated_at", "size"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 🔥 权限控制和CronJob过滤：根据view_all_teams参数和用户权限决定查询范围
	viewAllTeams := c.Query("view_all_teams") == "true"
	cronjobID := c.Query("cronjob") // 🔥 CronJob过滤参数
	month := c.Query("month")       // 🔥🔥 月份过滤参数（格式：202510）

	// 🚀 如果未指定month，默认使用当前年月（最常用场景）
	if month == "" {
		month = time.Now().Format("200601") // 格式：202510
		logger.Debug("month参数为空，使用当前年月",
			zap.String("month", month))
	}

	// 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	var taskLogs []*core.TaskLog
	var total int64
	var err error

	// 🔥 确定查询的团队范围
	var teamIDs []string
	if viewAllTeams {
		// 查看用户所属的所有团队数据
		userTeamIDs, exists := controller.GetUserTeamIDs(c)
		if !exists || len(userTeamIDs) == 0 {
			// 用户没有团队，返回空结果
			taskLogs = []*core.TaskLog{}
			total = 0
			goto BuildResponse
		}
		teamIDs = userTeamIDs
	} else {
		// 查看当前团队数据
		currentTeamID, exists := controller.GetCurrentTeamID(c)
		if !exists || currentTeamID == "" {
			// 没有当前团队，返回空结果
			taskLogs = []*core.TaskLog{}
			total = 0
			goto BuildResponse
		}
		teamIDs = []string{currentTeamID}
	}

	// 🔥🔥 统一使用 ListByTeamsAndCronjob 方法（支持 cronjobID 为空）
	// cronjobID 不为空: 过滤特定CronJob的TaskLog
	// cronjobID 为空: 查询该团队的所有TaskLog
	if shardService, ok := controller.service.(interface {
		ListByTeamsAndCronjob(ctx context.Context, teamIDs []string, cronjobID string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error)
		CountByTeamsAndCronjob(ctx context.Context, teamIDs []string, cronjobID string, filterActions ...filters.Filter) (int64, error)
	}); ok {
		// 🚀🚀 P1优化：如果指定了month参数，注入到context（格式：202510）
		// 性能提升：只查询指定月份的表，提升10倍+
		ctx := c.Request.Context()
		if month != "" {
			ctx = store.WithMonth(ctx, month)
			logger.Debug("使用月份参数优化查询",
				zap.String("month", month),
				zap.Strings("team_ids", teamIDs))
		} else if cronjobID != "" {
			logger.Debug("使用CronJob子查询优化方法",
				zap.String("cronjob", cronjobID),
				zap.Strings("team_ids", teamIDs))
		}

		taskLogs, err = shardService.ListByTeamsAndCronjob(ctx, teamIDs, cronjobID, offset, pagination.PageSize, filterActions...)
		if err == nil {
			total, err = shardService.CountByTeamsAndCronjob(ctx, teamIDs, cronjobID, filterActions...)
		}
	} else {
		// 降级到原有的团队过滤方式
		logger.Warn("分片服务不支持优化查询方法，使用降级方案")
		if viewAllTeams {
			filterActions = controller.AppendUserTeamsFilter(c, filterActions)
		} else {
			filterActions = controller.AppendTeamFilterWithOptions(c, filterActions, false)
		}
		taskLogs, err = controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
		if err == nil {
			total, err = controller.service.Count(c.Request.Context(), filterActions...)
		}
	}

BuildResponse:

	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 7. 🔥 列表页不加载内容，只返回基本信息（性能优化：避免N次文件IO）
	var results []map[string]interface{}
	for _, taskLog := range taskLogs {
		item := map[string]interface{}{
			"task_id": taskLog.TaskID,
			"storage": taskLog.Storage,
			"path":    taskLog.Path,
			// "content":    "", // 🔥 列表页不返回内容，需要内容时调用详情接口
			"size":       taskLog.Size,
			"created_at": taskLog.CreatedAt,
			"updated_at": taskLog.UpdatedAt,
		}
		results = append(results, item)
	}

	// 8. 构建分页结果
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  results,
	}

	// 9. 返回结果
	controller.HandleOK(c, result)
}

// 🔥🔥 parseOptimizationContext 解析URL参数并创建包含优化信息的Context
// 这是一个优雅的方式，避免在每个方法中重复解析参数
func (controller *TaskLogController) parseOptimizationContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()

	// 解析优化参数
	var createdAt, startTime, endTime *time.Time

	// 解析精确创建时间（优先级最高，性能最优）
	if createdAtStr := c.Query("created_at"); createdAtStr != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
			createdAt = &t
		} else if t, err := time.Parse("2006-01-02", createdAtStr); err == nil {
			createdAt = &t
		}
	}

	// 解析时间范围（当没有精确时间时使用）
	if createdAt == nil {
		if startTimeStr := c.Query("start_time"); startTimeStr != "" {
			if t, err := time.Parse("2006-01-02", startTimeStr); err == nil {
				startTime = &t
			}
		}
		if endTimeStr := c.Query("end_time"); endTimeStr != "" {
			if t, err := time.Parse("2006-01-02", endTimeStr); err == nil {
				// 结束时间设为当天的23:59:59
				endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				endTime = &endOfDay
			}
		}
	}

	// 如果有优化信息，则添加到context中
	if createdAt != nil || startTime != nil || endTime != nil {
		opt := &store.TaskLogOptimization{
			CreatedAt: createdAt,
			StartTime: startTime,
			EndTime:   endTime,
		}
		ctx = store.WithTaskLogOptimization(ctx, opt)
	}

	return ctx
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

// validateTaskLogAccess 验证用户是否有权限访问指定的TaskLog
// 🔥 更优雅的方案：利用分片服务的团队过滤功能来验证权限
func (controller *TaskLogController) validateTaskLogAccess(c *gin.Context, taskID string) error {
	// 🔥 管理员可以访问所有TaskLog
	if controller.IsAdmin(c) {
		return nil
	}

	// 🔥 获取用户的团队ID列表
	userTeamIDs, exists := controller.GetUserTeamIDs(c)
	if !exists || len(userTeamIDs) == 0 {
		return fmt.Errorf("用户没有团队权限")
	}

	// 🔥 使用分片服务的团队过滤功能来验证权限
	// 如果用户有权限访问该TaskLog，那么通过团队过滤应该能查询到它
	if shardService, ok := controller.service.(interface {
		ListByTeams(ctx context.Context, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error)
	}); ok {
		// 构建精确的TaskID过滤器
		taskIDFilter := &filters.FilterOption{
			QueryKey: "task_id",
			Column:   "task_id",
			Op:       filters.FILTER_EQ,
			Value:    taskID,
		}

		// 通过用户的团队ID列表查询该TaskLog
		taskLogs, err := shardService.ListByTeams(c.Request.Context(), userTeamIDs, 0, 1, taskIDFilter)
		if err != nil {
			return fmt.Errorf("验证TaskLog权限失败: %w", err)
		}

		// 如果查询结果为空，说明用户无权限访问
		if len(taskLogs) == 0 {
			return fmt.Errorf("用户无权限访问该TaskLog")
		}

		// 查询到结果，说明用户有权限
		return nil
	}

	// 如果服务不支持团队过滤，降级到基础权限验证
	// 这种情况下我们只能允许访问，因为无法精确验证
	return nil
}
