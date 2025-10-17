package controllers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// AuditUsageExample 审计功能完整使用示例
// 展示如何在Gin Web应用中使用audit.go进行审计日志记录
type AuditUsageExample struct {
	BaseController
	db           *gorm.DB
	auditService AuditService
}

// NewAuditUsageExample 创建审计使用示例
func NewAuditUsageExample() *AuditUsageExample {
	return &AuditUsageExample{}
}

// ==================== 应用启动配置 ====================

// SetupDatabase 设置数据库连接
func (example *AuditUsageExample) SetupDatabase(dsn string) error {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	example.db = db
	log.Println("✅ 数据库连接成功")
	return nil
}

// SetupAuditService 设置审计服务
// 根据环境选择不同的审计配置
func (example *AuditUsageExample) SetupAuditService(env string) {
	if example.db == nil {
		log.Fatal("❌ 数据库未初始化")
	}

	switch env {
	case "development":
		// 开发环境：同步数据库保存，确保数据一致性
		example.auditService = NewDatabaseAuditService(example.db, false)
		log.Println("🔧 开发环境审计服务已配置（同步模式）")

	case "production":
		// 生产环境：异步数据库保存，提高性能
		example.auditService = NewDatabaseAuditService(example.db, true)
		log.Println("🚀 生产环境审计服务已配置（异步模式）")

	default:
		// 自定义配置
		config := &AuditConfig{
			Async:         true,
			BatchSize:     100,
			MaxRetries:    3,
			RetryInterval: time.Second,
			Hook:          NewDatabaseAuditHook(example.db),
		}
		example.auditService = NewAuditService(config, example.db)
		log.Println("⚙️ 自定义审计服务已配置")
	}

	// 设置全局审计服务
	SetAuditService(example.auditService)
}

// ==================== 控制器方法示例 ====================

// CreateTask 创建任务
// 展示如何在创建操作中记录审计日志
func (example *AuditUsageExample) CreateTask(c *gin.Context) {
	// 1. 解析请求数据
	var taskData struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Project     string `json:"project"`
	}

	if err := c.ShouldBindJSON(&taskData); err != nil {
		// 记录错误审计日志
		example.SetAuditLog(c, "error", map[string]interface{}{
			"error":     "请求数据解析失败",
			"details":   err.Error(),
			"operation": "create_task",
		}, true)
		example.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 模拟创建任务
	taskID := "task_" + time.Now().Format("20060102150405")

	// 3. 记录创建操作的审计日志
	example.LogCreateAudit(c, "task", taskID, map[string]interface{}{
		"name":        taskData.Name,
		"description": taskData.Description,
		"project":     taskData.Project,
		"created_by":  c.GetHeader("X-User-ID"),
	})

	// 4. 返回成功响应
	example.HandleCreated(c, gin.H{
		"id":      taskID,
		"message": "任务创建成功",
		"data":    taskData,
	})
}

// UpdateTask 更新任务
// 展示如何在更新操作中记录审计日志
func (example *AuditUsageExample) UpdateTask(c *gin.Context) {
	taskID := c.Param("id")

	// 1. 解析更新数据
	var updateData struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		example.SetAuditLog(c, "error", map[string]interface{}{
			"error":   "更新数据解析失败",
			"task_id": taskID,
			"details": err.Error(),
		}, true)
		example.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 模拟更新任务
	// 这里应该调用实际的业务逻辑

	// 3. 记录更新操作的审计日志
	example.LogUpdateAudit(c, "task", taskID, map[string]interface{}{
		"updated_fields": updateData,
		"updated_by":     c.GetHeader("X-User-ID"),
		"updated_at":     time.Now().Format(time.RFC3339),
	})

	// 4. 返回成功响应
	example.HandleOK(c, gin.H{
		"id":      taskID,
		"message": "任务更新成功",
		"data":    updateData,
	})
}

// DeleteTask 删除任务
// 展示如何在删除操作中记录审计日志
func (example *AuditUsageExample) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	// 1. 模拟删除任务
	// 这里应该调用实际的业务逻辑

	// 2. 记录删除操作的审计日志
	example.LogDeleteAudit(c, "task", taskID, map[string]interface{}{
		"deleted_by": c.GetHeader("X-User-ID"),
		"deleted_at": time.Now().Format(time.RFC3339),
		"reason":     "用户主动删除",
	})

	// 3. 返回成功响应
	example.HandleNoContent(c)
}

// GetTask 获取任务详情
// 展示如何在读取操作中记录审计日志
func (example *AuditUsageExample) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	// 1. 模拟获取任务数据
	taskData := map[string]interface{}{
		"id":          taskID,
		"name":        "示例任务",
		"description": "这是一个示例任务",
		"status":      "active",
		"created_at":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		"updated_at":  time.Now().Format(time.RFC3339),
	}

	// 2. 记录读取操作的审计日志
	example.LogReadAudit(c, "task", taskID, map[string]interface{}{
		"accessed_by": c.GetHeader("X-User-ID"),
		"accessed_at": time.Now().Format(time.RFC3339),
	})

	// 3. 返回成功响应
	example.HandleOK(c, taskData)
}

// ListTasks 获取任务列表
// 展示如何在列表查询中记录审计日志
func (example *AuditUsageExample) ListTasks(c *gin.Context) {
	// 1. 解析分页参数
	pagination := example.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		{QueryKey: "status", Column: "status", Op: filters.FILTER_EQ},
		{QueryKey: "project", Column: "project", Op: filters.FILTER_EQ},
		{QueryKey: "name__contains", Column: "name", Op: filters.FILTER_CONTAINS},
	}

	// 3. 定义搜索字段
	searchFields := []string{"name", "description"}

	// 4. 定义排序字段
	orderingFields := []string{"created_at", "updated_at", "name"}
	defaultOrdering := "-created_at"

	// 5. 获取过滤动作
	_ = example.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 模拟查询任务列表
	// 这里应该调用实际的业务逻辑
	tasks := []map[string]interface{}{
		{"id": "task_1", "name": "任务1", "status": "active"},
		{"id": "task_2", "name": "任务2", "status": "completed"},
	}
	totalCount := int64(len(tasks))

	// 7. 记录列表查询的审计日志
	example.LogReadAudit(c, "task_list", "", map[string]interface{}{
		"page":        pagination.Page,
		"page_size":   pagination.PageSize,
		"total":       totalCount,
		"filters":     c.Request.URL.Query(),
		"searched_by": c.GetHeader("X-User-ID"),
	})

	// 8. 构建分页结果
	result := gin.H{
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
		"count":     totalCount,
		"results":   tasks,
	}

	// 9. 返回成功响应
	example.HandleOK(c, result)
}

// UserLogin 用户登录
// 展示如何记录用户登录的审计日志
func (example *AuditUsageExample) UserLogin(c *gin.Context) {
	// 1. 解析登录数据
	var loginData struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginData); err != nil {
		example.SetAuditLog(c, "error", map[string]interface{}{
			"error":    "登录数据解析失败",
			"username": loginData.Username,
			"details":  err.Error(),
		}, true)
		example.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 模拟用户验证
	// 这里应该调用实际的认证逻辑
	userID := "user_" + loginData.Username
	loginSuccess := true

	// 3. 记录登录操作的审计日志
	auditData := map[string]interface{}{
		"username":   loginData.Username,
		"user_id":    userID,
		"ip_address": c.ClientIP(),
		"user_agent": c.GetHeader("User-Agent"),
		"login_time": time.Now().Format(time.RFC3339),
		"success":    loginSuccess,
	}

	if loginSuccess {
		example.LogAudit(c, AuditActionLogin, "user", userID, auditData)
		example.HandleOK(c, gin.H{
			"message": "登录成功",
			"user_id": userID,
			"token":   "jwt_token_here",
		})
	} else {
		auditData["error"] = "用户名或密码错误"
		example.SetAuditLog(c, "error", auditData, true)
		example.HandleError(c, errors.New("用户名或密码错误"), http.StatusUnauthorized)
	}
}

// UserLogout 用户登出
// 展示如何记录用户登出的审计日志
func (example *AuditUsageExample) UserLogout(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	username := c.GetHeader("X-Username")

	// 记录登出操作的审计日志
	example.LogAudit(c, AuditActionLogout, "user", userID, map[string]interface{}{
		"username":    username,
		"user_id":     userID,
		"logout_time": time.Now().Format(time.RFC3339),
		"ip_address":  c.ClientIP(),
	})

	example.HandleOK(c, gin.H{
		"message": "登出成功",
	})
}

// ==================== 审计日志查询和管理 ====================

// GetAuditLogs 查询审计日志
// 展示如何查询审计日志
func (example *AuditUsageExample) GetAuditLogs(c *gin.Context) {
	if example.db == nil {
		example.HandleError(c, errors.New("数据库未初始化"), http.StatusInternalServerError)
		return
	}

	// 1. 解析查询参数
	userID := c.Query("user_id")
	action := c.Query("action")
	resource := c.Query("resource")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 2. 构建查询
	query := example.db.Model(&AuditLog{})

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	// 3. 查询总数
	var total int64
	query.Count(&total)

	// 4. 查询数据
	var logs []AuditLog
	offset := (page - 1) * pageSize
	result := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs)

	if result.Error != nil {
		example.HandleError(c, result.Error, http.StatusInternalServerError)
		return
	}

	// 5. 返回结果
	example.HandleOK(c, gin.H{
		"page":      page,
		"page_size": pageSize,
		"count":     total,
		"results":   logs,
	})
}

// GetAuditStatistics 获取审计统计信息
// 展示如何分析审计日志数据
func (example *AuditUsageExample) GetAuditStatistics(c *gin.Context) {
	if example.db == nil {
		example.HandleError(c, errors.New("数据库未初始化"), http.StatusInternalServerError)
		return
	}

	// 1. 统计各种操作的数量
	var actionStats []struct {
		Action string `json:"action"`
		Count  int64  `json:"count"`
	}
	example.db.Model(&AuditLog{}).
		Select("action, count(*) as count").
		Group("action").
		Find(&actionStats)

	// 2. 统计用户活动
	var userStats []struct {
		UserID string `json:"user_id"`
		Count  int64  `json:"count"`
	}
	example.db.Model(&AuditLog{}).
		Select("user_id, count(*) as count").
		Group("user_id").
		Order("count DESC").
		Limit(10).
		Find(&userStats)

	// 3. 统计成功率
	var successStats []struct {
		Success bool  `json:"success"`
		Count   int64 `json:"count"`
	}
	example.db.Model(&AuditLog{}).
		Select("success, count(*) as count").
		Group("success").
		Find(&successStats)

	// 4. 返回统计结果
	example.HandleOK(c, gin.H{
		"action_stats":  actionStats,
		"user_stats":    userStats,
		"success_stats": successStats,
	})
}

// ==================== 路由注册 ====================

// RegisterRoutes 注册路由
// 展示如何将审计功能集成到Gin路由中
func (example *AuditUsageExample) RegisterRoutes(router *gin.Engine) {
	// API路由组
	api := router.Group("/api/v1")
	{
		// 任务相关路由
		tasks := api.Group("/tasks")
		{
			tasks.POST("", example.CreateTask)       // 创建任务
			tasks.GET("", example.ListTasks)         // 获取任务列表
			tasks.GET("/:id", example.GetTask)       // 获取任务详情
			tasks.PUT("/:id", example.UpdateTask)    // 更新任务
			tasks.DELETE("/:id", example.DeleteTask) // 删除任务
		}

		// 用户相关路由
		users := api.Group("/users")
		{
			users.POST("/login", example.UserLogin)   // 用户登录
			users.POST("/logout", example.UserLogout) // 用户登出
		}

		// 审计相关路由
		audit := api.Group("/audit")
		{
			audit.GET("/logs", example.GetAuditLogs)             // 查询审计日志
			audit.GET("/statistics", example.GetAuditStatistics) // 获取审计统计
		}
	}
}

// ==================== 应用启动示例 ====================

// StartApplication 启动应用示例
// 展示如何完整地启动一个带有审计功能的Gin应用
func StartApplication() {
	// 1. 创建审计使用示例
	auditExample := NewAuditUsageExample()

	// 2. 设置数据库连接
	err := auditExample.SetupDatabase("user:password@tcp(localhost:3306)/audit_db?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		log.Fatal("❌ 数据库连接失败:", err)
	}

	// 3. 设置审计服务
	env := "production" // 或 "development"
	auditExample.SetupAuditService(env)

	// 4. 创建Gin引擎
	router := gin.Default()

	// 5. 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 6. 注册路由
	auditExample.RegisterRoutes(router)

	// 7. 启动服务器
	log.Println("🚀 服务器启动在 :8080")
	log.Println("📊 审计功能已启用")

	if err := router.Run(":8080"); err != nil {
		log.Fatal("❌ 服务器启动失败:", err)
	}
}

// ==================== 使用说明 ====================

/*
使用说明：

1. 数据库配置：
   - 确保MySQL数据库已启动
   - 修改数据库连接字符串
   - 审计日志表会自动创建

2. 环境配置：
   - development: 同步模式，确保数据一致性
   - production: 异步模式，提高性能

3. 审计日志记录：
   - 自动记录所有CRUD操作
   - 支持自定义审计日志
   - 支持错误审计日志

4. 查询审计日志：
   - GET /api/v1/audit/logs - 查询审计日志
   - GET /api/v1/audit/statistics - 获取统计信息

5. 测试接口：
   - POST /api/v1/tasks - 创建任务
   - GET /api/v1/tasks - 获取任务列表
   - GET /api/v1/tasks/:id - 获取任务详情
   - PUT /api/v1/tasks/:id - 更新任务
   - DELETE /api/v1/tasks/:id - 删除任务
   - POST /api/v1/users/login - 用户登录
   - POST /api/v1/users/logout - 用户登出

6. 请求头设置：
   - X-User-ID: 用户ID
   - X-Username: 用户名
   - X-Request-ID: 请求ID（可选）

示例请求：
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user123" \
  -H "X-Username: admin" \
  -d '{"name":"测试任务","description":"这是一个测试任务","project":"test"}'
*/
