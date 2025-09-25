// Package app 应用程序核心模块
//
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/middleware"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// initRouter 初始化所有API路由
//
// 此函数负责设置整个API服务器的路由结构，包括：
// 1. 基础路由（健康检查、根路径等）
// 2. API v1路由组，包含所有业务接口
// 3. 数据库连接和自动迁移
// 4. Session配置
// 5. 各业务模块的路由注册：
//   - 用户管理 (/api/v1/user/)
//   - 工作节点管理 (/api/v1/worker/)
//   - 分类管理 (/api/v1/category/)
//   - 定时任务管理 (/api/v1/cronjob/)
//   - 任务记录管理 (/api/v1/task/)
//   - 分布式锁管理 (/api/v1/lock/)
//   - WebSocket连接 (/api/v1/ws/task/)
//   - 健康检查 (/api/v1/health/)
//
// 参数:
//   - app: Gin引擎实例，用于注册路由
func initRouter(app *gin.Engine) {
	// 根路径 - 系统状态检查
	app.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "计划任务系统 API Server 运行正常",
			"version": "1.0.0",
			"status":  "running",
		})
	})

	// 创建API v1路由组
	// 所有业务接口都挂载在 /api/v1 路径下
	apis := app.Group("/api/v1")

	// 初始化数据库连接
	db, err := core.GetDB()
	if err != nil {
		logger.Panic("数据库连接失败", zap.Error(err))
		return
	} else {
		// 执行数据库自动迁移
		// 确保所有表结构都是最新的
		if err := core.AutoMigrate(db); err != nil {
			logger.Panic("数据库自动迁移失败", zap.Error(err))
			return
		}
		logger.Info("数据库连接和迁移完成")
	}

	// 配置Session存储
	// 当前使用Cookie存储，生产环境建议使用Redis或数据库存储
	// 注释掉的代码是使用数据库存储Session的配置
	//sstore := gormsessions.NewStore(db, true, []byte(config.Web.SessionSecretKey))
	sstore := cookie.NewStore([]byte(config.Web.SessionSecretKey))

	// 配置Session选项
	sstore.Options(sessions.Options{
		Secure:   true,          // 仅HTTPS传输
		SameSite: 5,             // SameSite=Lax，防止CSRF攻击
		Path:     "/",           // 所有路径都可用
		MaxAge:   3600 * 24 * 7, // 7天过期
	})

	// 为API路由组添加Session中间件
	apis.Use(sessions.Sessions(config.Web.SessionIDName, sstore))

	// 添加Prometheus监控中间件
	apis.Use(middleware.PrometheusMiddleware())        // 基础HTTP监控
	apis.Use(middleware.MetricsCollectionMiddleware()) // 业务指标收集
	apis.Use(middleware.DatabaseMetricsMiddleware())   // 数据库操作监控

	// ========== 用户管理模块 ==========
	// 提供用户账户的CRUD操作
	// 注意：根据要求，用户模块暂时不处理
	s := store.NewUserStore(db)
	userService := services.NewUserService(s)
	userControler := controllers.NewUserController(userService)
	apis.POST("/user/", userControler.Create)       // 创建用户
	apis.GET("/user/", userControler.List)          // 获取用户列表
	apis.GET("/user/:id/", userControler.Find)      // 根据ID获取用户
	apis.PUT("/user/:id/", userControler.Update)    // 更新用户信息
	apis.DELETE("/user/:id/", userControler.Delete) // 删除用户

	// ========== 工作节点管理模块 ==========
	// 管理工作节点（Worker）的注册、状态监控等
	// Worker是执行具体任务的节点，通过心跳保持连接状态
	workerStore := store.NewWorkerStore(db)
	workerService := services.NewWorkerService(workerStore)
	workerController := controllers.NewWorkerController(workerService)
	apis.POST("/worker/", workerController.Create)       // 注册新的工作节点
	apis.GET("/worker/", workerController.List)          // 获取工作节点列表
	apis.GET("/worker/:id/", workerController.Find)      // 根据ID获取工作节点信息
	apis.PUT("/worker/:id/", workerController.Update)    // 更新工作节点信息
	apis.DELETE("/worker/:id/", workerController.Delete) // 注销工作节点
	apis.GET("/worker/:id/ping/", workerController.Ping) // 工作节点心跳接口，用于保持连接状态

	// ========== 分类管理模块 ==========
	// 管理任务分类，用于对定时任务进行分类管理
	// 分类可以帮助组织和管理不同类型的任务
	categoryStore := store.NewCategoryStore(db)
	categoryService := services.NewCategoryService(categoryStore)
	categoryController := controllers.NewCategoryController(categoryService)
	apis.POST("/category/", categoryController.Create)       // 创建分类
	apis.GET("/category/", categoryController.List)          // 获取分类列表
	apis.GET("/category/:id/", categoryController.Find)      // 根据ID获取分类
	apis.PUT("/category/:id/", categoryController.Update)    // 更新分类信息
	apis.DELETE("/category/:id/", categoryController.Delete) // 删除分类

	// ========== 定时任务管理模块 ==========
	// 核心模块：管理定时任务的定义、调度和执行
	// 支持cron表达式，可以创建复杂的定时调度规则
	cronjobStore := store.NewCronJobStore(db)
	cronjobService := services.NewCronJobService(cronjobStore)
	cronjobController := controllers.NewCronJobController(cronjobService)
	apis.POST("/cronjob/", cronjobController.Create)                                          // 创建定时任务
	apis.GET("/cronjob/", cronjobController.List)                                             // 获取定时任务列表
	apis.GET("/cronjob/:id/", cronjobController.Find)                                         // 根据ID获取定时任务
	apis.PUT("/cronjob/:id/", cronjobController.Update)                                       // 更新定时任务信息
	apis.DELETE("/cronjob/:id/", cronjobController.Delete)                                    // 删除定时任务
	apis.PUT("/cronjob/:id/toggle-active/", cronjobController.ToggleActive)                   // 切换任务激活状态（启用/禁用）
	apis.POST("/cronjob/validate-expression/", cronjobController.ValidateExpression)          // 验证cron表达式是否有效
	apis.GET("/cronjob/project/:project/name/:name/", cronjobController.FindByProjectAndName) // 根据项目和名称获取定时任务
	apis.PATCH("/cronjob/:id/", cronjobController.Patch)                                      // 动态更新定时任务的部分字段

	// ========== 分布式锁管理模块 ==========
	// 基于Redis的分布式锁，确保任务不重复执行
	// 在分布式环境下，防止多个节点同时执行同一个任务
	lockerService, err := services.NewRedisLocker()
	if err != nil {
		logger.Panic("创建Redis分布式锁服务失败", zap.Error(err))
	}
	lockController := controllers.NewLockController(lockerService)
	apis.GET("/lock/acquire", lockController.Acquire) // 获取分布式锁
	apis.GET("/lock/release", lockController.Release) // 释放分布式锁
	apis.GET("/lock/check", lockController.Check)     // 检查锁状态
	apis.GET("/lock/refresh", lockController.Refresh) // 刷新锁的过期时间

	// ========== 任务执行记录模块 ==========
	// 记录每次任务执行的详细信息，包括状态、输出、时间等
	// 这是定时任务执行后产生的具体任务实例
	taskStore := store.NewTaskStore(db)
	taskService := services.NewTaskService(taskStore)
	taskController := controllers.NewTaskController(taskService)
	apis.POST("/task/", taskController.Create)                        // 创建任务记录
	apis.GET("/task/", taskController.List)                           // 获取任务记录列表
	apis.GET("/task/:id/", taskController.Find)                       // 根据ID获取任务记录
	apis.PUT("/task/:id/", taskController.Update)                     // 更新任务记录
	apis.DELETE("/task/:id/", taskController.Delete)                  // 删除任务记录
	apis.PUT("/task/:id/update-status/", taskController.UpdateStatus) // 更新任务执行状态
	apis.PUT("/task/:id/update-output/", taskController.UpdateOutput) // 更新任务执行输出
	apis.PATCH("/task/:id/", taskController.Patch)                    // 动态更新任务记录的部分字段

	// ========== 任务日志管理模块 ==========
	// 管理任务执行的详细日志，支持多种存储方式（数据库、文件、S3）
	// 提供日志的增删改查、内容读写等功能
	taskLogStore := store.NewTaskLogStore(db)
	taskLogService := services.NewTaskLogService(taskLogStore)
	taskLogController := controllers.NewTaskLogController(taskLogService)
	apis.POST("/tasklog/", taskLogController.Create)                        // 创建任务日志
	apis.GET("/tasklog/", taskLogController.List)                           // 获取任务日志列表
	apis.GET("/tasklog/:task_id/", taskLogController.Find)                  // 根据任务ID获取任务日志
	apis.PUT("/tasklog/:task_id/", taskLogController.Update)                // 更新任务日志
	apis.DELETE("/tasklog/:task_id/", taskLogController.Delete)             // 删除任务日志
	apis.GET("/tasklog/:task_id/content/", taskLogController.GetContent)    // 获取任务日志内容
	apis.PUT("/tasklog/:task_id/content/", taskLogController.SaveContent)   // 保存任务日志内容
	apis.POST("/tasklog/:task_id/append/", taskLogController.AppendContent) // 追加任务日志内容

	// ========== WebSocket实时通信模块 ==========
	// 提供与Worker节点的实时通信能力
	// 用于任务分发、状态同步、实时监控等
	websocketService := services.NewWebsocketService(taskStore, workerStore)
	websocketController := controllers.NewWebsocketController(websocketService)
	// WebSocket连接接口，不使用中间件（特别是认证中间件）
	// 因为Worker节点需要通过WebSocket进行实时通信
	apis.GET("/ws/task/", websocketController.HandleConnect)

	// ========== 系统健康检查模块 ==========
	// 提供系统状态监控和健康检查功能
	// 用于监控系统各组件的工作状态
	healthController := controllers.NewHealthController(websocketService, taskService)
	apis.GET("/health/", healthController.Health)

	// ========== 监控指标模块 ==========
	// 提供Prometheus监控指标端点
	// 用于监控系统性能和业务指标
	metricsController := controllers.NewMetricsController()
	app.GET("/metrics", metricsController.Metrics) // 注意：直接注册到app，不经过apis路由组

	// 为了安全考虑，也可以将metrics端点放在单独的端口
	// 或者添加认证中间件保护

	logger.Info("所有API路由初始化完成")
}
