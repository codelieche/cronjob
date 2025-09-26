// Package app 应用程序核心模块
//
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"net/http"

	_ "github.com/codelieche/cronjob/apiserver/docs" // 导入生成的 Swagger 文档
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
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// Swagger 文档路由
	app.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		Secure:   false,         // 开发环境可以设为false，生产环境应设为true
		HttpOnly: true,          // 防止XSS攻击
		SameSite: 3,             // SameSite=Lax，防止CSRF攻击 (http.SameSiteLaxMode = 3)
		Path:     "/",           // 所有路径都可用
		MaxAge:   3600 * 24 * 7, // 7天过期
	})

	// 为API路由组添加Session中间件
	apis.Use(sessions.Sessions(config.Web.SessionIDName, sstore))

	// 添加Prometheus监控中间件
	apis.Use(middleware.PrometheusMiddleware())        // 基础HTTP监控
	apis.Use(middleware.MetricsCollectionMiddleware()) // 业务指标收集
	apis.Use(middleware.DatabaseMetricsMiddleware())   // 数据库操作监控

	// ========== 创建认证中间件组合 ==========
	// 使用新的模块化认证中间件设计
	authGroup := middleware.NewAuthMiddlewareGroup()

	// ========== 工作节点管理模块 ==========
	// 管理工作节点（Worker）的注册、状态监控等
	// Worker节点可能有自己的认证机制，暂时不使用HTTP认证中间件
	// 后续可以根据需要添加专门的Worker认证中间件
	workerStore := store.NewWorkerStore(db)
	workerService := services.NewWorkerService(workerStore)
	workerController := controllers.NewWorkerController(workerService)

	// Worker接口暂时不使用认证中间件（根据业务需求可调整）
	workerRoutes := apis.Group("/worker")
	// 如果需要为Worker添加认证，可以使用：
	workerRoutes.Use(authGroup.Standard)
	{
		workerRoutes.POST("/", workerController.Create)       // 注册新的工作节点
		workerRoutes.GET("/", workerController.List)          // 获取工作节点列表
		workerRoutes.GET("/:id/", workerController.Find)      // 根据ID获取工作节点信息
		workerRoutes.PUT("/:id/", workerController.Update)    // 更新工作节点信息
		workerRoutes.DELETE("/:id/", workerController.Delete) // 注销工作节点
		workerRoutes.GET("/:id/ping/", workerController.Ping) // 工作节点心跳接口
	}

	// ========== 分类管理模块 ==========
	// 管理任务分类，需要用户认证
	categoryStore := store.NewCategoryStore(db)
	categoryService := services.NewCategoryService(categoryStore)
	categoryController := controllers.NewCategoryController(categoryService)

	// 分类管理接口需要用户认证
	categoryRoutes := apis.Group("/category")
	categoryRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		categoryRoutes.POST("/", categoryController.Create)       // 创建分类
		categoryRoutes.GET("/", categoryController.List)          // 获取分类列表
		categoryRoutes.GET("/:id/", categoryController.Find)      // 根据ID获取分类
		categoryRoutes.PUT("/:id/", categoryController.Update)    // 更新分类信息
		categoryRoutes.DELETE("/:id/", categoryController.Delete) // 删除分类
	}

	// ========== 定时任务管理模块 ==========
	// 核心模块：管理定时任务的定义、调度和执行，需要用户认证
	cronjobStore := store.NewCronJobStore(db)
	cronjobService := services.NewCronJobService(cronjobStore)
	cronjobController := controllers.NewCronJobController(cronjobService)

	// 定时任务管理接口需要用户认证
	cronjobRoutes := apis.Group("/cronjob")
	cronjobRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		cronjobRoutes.POST("/", cronjobController.Create)                                          // 创建定时任务
		cronjobRoutes.GET("/", cronjobController.List)                                             // 获取定时任务列表
		cronjobRoutes.GET("/:id/", cronjobController.Find)                                         // 根据ID获取定时任务
		cronjobRoutes.PUT("/:id/", cronjobController.Update)                                       // 更新定时任务信息
		cronjobRoutes.DELETE("/:id/", cronjobController.Delete)                                    // 删除定时任务
		cronjobRoutes.PUT("/:id/toggle-active/", cronjobController.ToggleActive)                   // 切换任务激活状态
		cronjobRoutes.POST("/validate-expression/", cronjobController.ValidateExpression)          // 验证cron表达式
		cronjobRoutes.GET("/project/:project/name/:name/", cronjobController.FindByProjectAndName) // 根据项目和名称获取任务
		cronjobRoutes.PATCH("/:id/", cronjobController.Patch)                                      // 动态更新部分字段
	}

	// ========== 分布式锁管理模块 ==========
	// 基于Redis的分布式锁，主要供Worker节点使用，暂时不使用认证中间件
	// 如果需要保护这些接口，可以添加专门的Worker认证机制
	lockerService, err := services.NewRedisLocker()
	if err != nil {
		logger.Panic("创建Redis分布式锁服务失败", zap.Error(err))
	}
	lockController := controllers.NewLockController(lockerService)

	// 分布式锁接口暂时不使用认证中间件（主要供Worker使用）
	lockRoutes := apis.Group("/lock")
	// 如果需要为分布式锁添加认证，可以使用：
	lockRoutes.Use(authGroup.Standard)
	{
		lockRoutes.GET("/acquire", lockController.Acquire) // 获取分布式锁
		lockRoutes.GET("/release", lockController.Release) // 释放分布式锁
		lockRoutes.GET("/check", lockController.Check)     // 检查锁状态
		lockRoutes.GET("/refresh", lockController.Refresh) // 刷新锁的过期时间
	}

	// ========== 任务执行记录模块 ==========
	// 记录每次任务执行的详细信息，需要用户认证
	taskStore := store.NewTaskStore(db)
	taskService := services.NewTaskService(taskStore)
	taskController := controllers.NewTaskController(taskService)

	// 任务记录管理接口需要用户认证
	taskRoutes := apis.Group("/task")
	taskRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		taskRoutes.POST("/", taskController.Create)                        // 创建任务记录
		taskRoutes.GET("/", taskController.List)                           // 获取任务记录列表
		taskRoutes.GET("/:id/", taskController.Find)                       // 根据ID获取任务记录
		taskRoutes.PUT("/:id/", taskController.Update)                     // 更新任务记录
		taskRoutes.DELETE("/:id/", taskController.Delete)                  // 删除任务记录
		taskRoutes.PUT("/:id/update-status/", taskController.UpdateStatus) // 更新任务执行状态
		taskRoutes.PUT("/:id/update-output/", taskController.UpdateOutput) // 更新任务执行输出
		taskRoutes.PATCH("/:id/", taskController.Patch)                    // 动态更新任务记录的部分字段
	}

	// ========== 任务日志管理模块 ==========
	// 管理任务执行的详细日志，需要用户认证
	taskLogStore := store.NewTaskLogStore(db)
	taskLogService := services.NewTaskLogService(taskLogStore)
	taskLogController := controllers.NewTaskLogController(taskLogService)

	// 任务日志管理接口需要用户认证
	taskLogRoutes := apis.Group("/tasklog")
	taskLogRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		taskLogRoutes.POST("/", taskLogController.Create)                        // 创建任务日志
		taskLogRoutes.GET("/", taskLogController.List)                           // 获取任务日志列表
		taskLogRoutes.GET("/:task_id/", taskLogController.Find)                  // 根据任务ID获取任务日志
		taskLogRoutes.PUT("/:task_id/", taskLogController.Update)                // 更新任务日志
		taskLogRoutes.DELETE("/:task_id/", taskLogController.Delete)             // 删除任务日志
		taskLogRoutes.GET("/:task_id/content/", taskLogController.GetContent)    // 获取任务日志内容
		taskLogRoutes.PUT("/:task_id/content/", taskLogController.SaveContent)   // 保存任务日志内容
		taskLogRoutes.POST("/:task_id/append/", taskLogController.AppendContent) // 追加任务日志内容
	}

	// ========== WebSocket实时通信模块 ==========
	// 提供与Worker节点的实时通信能力，不使用HTTP认证中间件
	// WebSocket有自己的认证机制
	websocketService := services.NewWebsocketService(taskStore, workerStore)
	websocketController := controllers.NewWebsocketController(websocketService)

	// WebSocket连接接口，不使用认证中间件（有自己的认证机制）
	wsRoutes := apis.Group("/ws")
	{
		wsRoutes.GET("/task/", websocketController.HandleConnect) // WebSocket连接
	}

	// ========== 系统健康检查模块 ==========
	// 系统健康检查，不需要认证（公共接口）
	healthController := controllers.NewHealthController(websocketService, taskService)

	// 健康检查接口不需要认证
	healthRoutes := apis.Group("/health")
	{
		healthRoutes.GET("/", healthController.Health) // 系统健康检查
	}

	// ========== 监控指标模块 ==========
	// Prometheus监控指标端点，不需要认证（但可以考虑在生产环境中保护）
	metricsController := controllers.NewMetricsController()

	// 监控指标直接注册到app根路由，不经过apis路由组，避免中间件影响
	app.GET("/metrics", metricsController.Metrics)

	// ========== 认证缓存管理接口 ==========
	// 提供认证缓存管理功能，需要管理员权限
	cacheRoutes := apis.Group("/auth-cache")
	cacheRoutes.Use(authGroup.Admin) // 需要管理员权限
	{
		// 清空认证缓存
		cacheRoutes.DELETE("/", func(c *gin.Context) {
			middleware.ClearAuthCache()
			c.JSON(200, gin.H{
				"code":    0,
				"message": "认证缓存已清空",
			})
		})

		// 获取认证缓存统计
		cacheRoutes.GET("/stats/", func(c *gin.Context) {
			stats := middleware.GetAuthCacheStats()
			c.JSON(200, gin.H{
				"code": 0,
				"data": stats,
			})
		})
	}

	logger.Info("所有API路由初始化完成",
		zap.String("认证服务地址", config.Auth.ApiUrl),
		zap.Bool("认证缓存启用", config.Auth.EnableCache),
		zap.Duration("认证超时", config.Auth.Timeout))
}
