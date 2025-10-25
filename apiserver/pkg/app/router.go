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
	"github.com/codelieche/cronjob/apiserver/pkg/shard"
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
//
// 返回值:
//   - *services.QueueMetrics: 队列健康度指标管理器（需要在后台启动）
func initRouter(app *gin.Engine) *services.QueueMetrics {
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
		return nil
	} else {
		// 执行数据库自动迁移
		// 确保所有表结构都是最新的
		if err := core.AutoMigrate(db); err != nil {
			logger.Panic("数据库自动迁移失败", zap.Error(err))
			return nil
		}
		logger.Info("数据库连接和迁移完成")

		// 注册系统分类
		// 自动注册 default, command, http, script, database, message 等核心分类
		if err := RegisterCategories(db); err != nil {
			logger.Error("注册系统分类失败", zap.Error(err))
			// 不阻塞启动，继续运行
		} else {
			logger.Info("系统分类注册完成")
		}
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
		workerRoutes.PUT("/:id/ping/", workerController.Ping) // 工作节点心跳接口（修正为PUT）
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
		categoryRoutes.GET("/all/", categoryController.All)       // 获取所有分类（不分页）
		categoryRoutes.GET("/:id/", categoryController.Find)      // 根据ID获取分类
		categoryRoutes.PUT("/:id/", categoryController.Update)    // 更新分类信息
		categoryRoutes.DELETE("/:id/", categoryController.Delete) // 删除分类
	}

	// ========== 凭证管理模块 ==========
	// 管理敏感凭证信息（密码、Token等），敏感字段自动加密，需要用户认证
	credentialStore := store.NewCredentialStore(db)
	credentialService := services.NewCredentialService(credentialStore)
	credentialController := controllers.NewCredentialController(credentialService)

	// 凭证管理接口需要用户认证
	credentialRoutes := apis.Group("/credentials")
	credentialRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		credentialRoutes.GET("/types/", credentialController.ListTypes)      // 获取所有凭证类型
		credentialRoutes.POST("/", credentialController.Create)              // 创建凭证
		credentialRoutes.GET("/", credentialController.List)                 // 获取凭证列表
		credentialRoutes.GET("/all/", credentialController.All)              // 获取所有凭证（不分页）
		credentialRoutes.GET("/:id/", credentialController.Find)             // 根据ID获取凭证
		credentialRoutes.PUT("/:id/", credentialController.Update)           // 更新凭证信息
		credentialRoutes.PATCH("/:id/", credentialController.Patch)          // 动态更新部分字段
		credentialRoutes.DELETE("/:id/", credentialController.Delete)        // 删除凭证
		credentialRoutes.POST("/:id/decrypt/", credentialController.Decrypt) // 解密凭证（需要特殊权限）
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
		cronjobRoutes.POST("/", cronjobController.Create) // 创建定时任务
		cronjobRoutes.GET("/", cronjobController.List)    // 获取定时任务列表

		// 具体路径（必须在 /:id/ 之前注册，避免路由冲突）
		cronjobRoutes.POST("/validate-expression/", cronjobController.ValidateExpression)          // 验证cron表达式
		cronjobRoutes.GET("/project/:project/name/:name/", cronjobController.FindByProjectAndName) // 根据项目和名称获取任务

		// 单个任务操作（动态路由放在最后）
		cronjobRoutes.GET("/:id/", cronjobController.Find)                       // 根据ID获取定时任务
		cronjobRoutes.PUT("/:id/", cronjobController.Update)                     // 更新定时任务信息
		cronjobRoutes.PATCH("/:id/", cronjobController.Patch)                    // 动态更新部分字段
		cronjobRoutes.DELETE("/:id/", cronjobController.Delete)                  // 删除定时任务
		cronjobRoutes.PUT("/:id/toggle-active/", cronjobController.ToggleActive) // 切换任务激活状态
		cronjobRoutes.POST("/:id/execute/", cronjobController.Execute)           // 手动执行定时任务
	}

	// ========== 工作流管理模块 ⭐ ==========
	// 工作流编排模块：管理工作流模板、执行实例、任务流转
	// 🔥 核心功能：
	//   1. Workflow 模板管理（创建、更新、删除、查询）
	//   2. WorkflowExecute 执行实例管理（触发执行、查询、取消）
	//   3. 自动任务流转（Task 完成后自动激活下一个）
	//   4. 参数传递（Variables + Template 替换）
	//   5. 环境锁定（确保所有步骤在同一 Worker 执行）
	workflowStore := store.NewWorkflowStore(db)
	workflowService := services.NewWorkflowService(workflowStore)
	workflowController := controllers.NewWorkflowController(workflowService)

	// 🔥 将 credentialService 和 cronJobService 注入到 workflowService 中
	// 用于一键创建Webhook定时任务功能
	if ws, ok := workflowService.(*services.WorkflowService); ok {
		ws.SetCredentialService(credentialService)
		ws.SetCronJobService(cronjobService)
	}

	// Workflow 模板管理接口需要用户认证
	workflowRoutes := apis.Group("/workflow")
	workflowRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		workflowRoutes.POST("/", workflowController.Create) // 创建工作流模板
		workflowRoutes.GET("/", workflowController.List)    // 获取工作流列表

		// 具体路径（必须在 /:id/ 之前注册，避免路由冲突）
		workflowRoutes.GET("/by-code/:code/", workflowController.FindByCode) // 根据Code获取工作流（用于快捷访问）

		// 单个工作流操作（动态路由放在最后）
		workflowRoutes.GET("/:id/", workflowController.Find)                        // 根据ID获取工作流详情
		workflowRoutes.PUT("/:id/", workflowController.Update)                      // 更新工作流模板
		workflowRoutes.DELETE("/:id/", workflowController.Delete)                   // 删除工作流
		workflowRoutes.POST("/:id/toggle-active/", workflowController.ToggleActive) // 切换激活状态
		workflowRoutes.GET("/:id/statistics/", workflowController.GetStatistics)    // 获取统计信息
	}

	// ========== 工作流 Webhook 触发模块 🔥 ==========
	// Webhook 触发接口：无需认证，通过 Token 验证
	// Webhook 管理接口：需要用户认证
	// 🔥 注意：workflowExecService 会在后面创建，这里先声明控制器，后面再初始化路由
	var webhookController *controllers.WorkflowWebhookController

	// ========== 工作流执行管理模块 ⭐ ==========
	// WorkflowExecute 执行实例管理
	// 注意：TaskStore 在后面创建，这里先声明，后面再初始化
	var taskStore core.TaskStore
	var workflowExecService core.WorkflowExecuteService

	// 这些会在 taskStore 创建后初始化
	// workflowExecStore := store.NewWorkflowExecuteStore(db)
	// workflowExecService = services.NewWorkflowExecuteService(workflowExecStore, workflowStore, taskStore)
	// workflowExecController := controllers.NewWorkflowExecuteController(workflowExecService)

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
	taskStore = store.NewTaskStore(db)                               // 🔥 这里使用之前声明的变量
	taskService := services.NewTaskService(taskStore, lockerService) // 🔥 注入lockerService用于取消功能

	// 🔥 提前创建 approvalStore（用于 WorkflowExecuteService 的依赖注入）
	approvalStore := store.NewApprovalStore(db)
	approvalRecordStore := store.NewApprovalRecordStore(db)

	// 🔥 创建 WorkflowExecute 相关服务（在 taskStore 和 approvalStore 创建后）⭐
	workflowExecStore := store.NewWorkflowExecuteStore(db)
	workflowExecService = services.NewWorkflowExecuteService(workflowExecStore, workflowStore, taskStore, approvalStore)

	workflowExecController := controllers.NewWorkflowExecuteController(workflowExecService)

	// 🔥 创建 Webhook 控制器（在 workflowExecService 创建后）⭐
	webhookController = controllers.NewWorkflowWebhookController(workflowService, workflowExecService)

	// WorkflowExecute 执行实例管理接口需要用户认证
	workflowExecRoutes := apis.Group("/workflow-execute")
	workflowExecRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		workflowExecRoutes.GET("/:id/", workflowExecController.Find)           // 根据ID获取执行实例
		workflowExecRoutes.GET("/", workflowExecController.List)               // 获取执行实例列表
		workflowExecRoutes.POST("/:id/cancel/", workflowExecController.Cancel) // 取消执行
		workflowExecRoutes.DELETE("/:id/", workflowExecController.Delete)      // 删除执行实例
	}

	// Workflow 执行相关路由（挂在 workflow 路由组下）
	// 注意：这里必须使用 :id 而不是 :workflow_id，避免与上面的 /:id/ 路由冲突
	{
		workflowRoutes.POST("/:id/execute/", workflowExecController.Execute)          // ⭐ 触发执行
		workflowRoutes.GET("/:id/executes/", workflowExecController.ListByWorkflowID) // 执行历史

		// 🔥 Webhook 管理接口（需要用户认证）
		workflowRoutes.PUT("/:id/webhook/toggle", webhookController.ToggleWebhook)          // 启用/禁用Webhook
		workflowRoutes.POST("/:id/webhook/regenerate", webhookController.RegenerateToken)   // 重新生成Token
		workflowRoutes.PUT("/:id/webhook/whitelist", webhookController.UpdateIPWhitelist)   // 更新IP白名单
		workflowRoutes.GET("/:id/webhook/info", webhookController.GetWebhookInfo)           // 获取Webhook信息
		workflowRoutes.GET("/:id/webhook/url", webhookController.GetWebhookFullURL)         // 获取完整Webhook URL
		workflowRoutes.POST("/:id/webhook/cronjob", webhookController.CreateWebhookCronJob) // 🔥 一键创建Webhook定时任务
	}

	// 🔥 Webhook 触发接口（无需认证，通过查询参数key传递Token）
	// 注意：必须在所有需要认证的路由之外单独注册，避免被认证中间件拦截
	webhookRoutes := apis.Group("/workflow")
	// 不添加认证中间件，允许外部系统直接访问
	{
		webhookRoutes.POST("/:id/webhook", webhookController.TriggerByWebhook) // 🔥 Webhook触发（?key=token）
	}

	// ========== Workflow统计分析模块 ⭐ ==========
	// 提供Workflow执行的统计分析功能
	// 🔥 核心功能：
	//   1. 执行成功率趋势（最近N天）
	//   2. 执行效率分析（平均时长、时长分布）
	//   3. Workflow排行榜（Top 10高频Workflow）
	//   4. 时间分布分析（按星期统计）
	//   5. 时间段对比（本周vs上周、本月vs上月）
	//   6. 手动聚合触发（补偿机制）
	workflowStatsStore := store.NewWorkflowStatsStore(db)
	workflowStatsService := services.NewWorkflowStatsService(db, workflowStatsStore, workflowExecStore, workflowStore)
	workflowStatsController := controllers.NewWorkflowStatsController(workflowStatsService)

	// Workflow统计分析接口（需要用户认证）
	apis.GET("/workflow/analysis/", authGroup.Standard, workflowStatsController.GetAnalysis)

	// Workflow统计聚合接口（需要管理员权限）
	apis.POST("/workflow/stats/aggregate/daily", authGroup.Admin, workflowStatsController.TriggerDailyAggregation)           // 手动触发每日聚合
	apis.POST("/workflow/stats/aggregate/historical", authGroup.Admin, workflowStatsController.TriggerHistoricalAggregation) // 手动触发历史聚合

	// 🔥 创建dispatchService用于任务调度和重试（注意：在taskController之前创建）
	dispatchService := services.NewDispatchService(cronjobStore, taskStore, lockerService)

	// 🔥 创建websocketService用于任务Stop/Kill功能（注意：在taskController之前创建）
	websocketService := services.NewWebsocketService(taskStore, workerStore)

	// 🔥 创建 TaskController，注入 WorkflowExecuteService 用于自动任务流转 ⭐
	taskController := controllers.NewTaskController(taskService, dispatchService, websocketService, workflowExecService)

	// 🔥 将 taskService 注入到 cronjobService 中，用于手动执行任务功能
	// 注意：必须在 taskService 创建后才能注入，避免 nil pointer
	if cs, ok := cronjobService.(*services.CronJobService); ok {
		cs.SetTaskService(taskService)
	}

	// 🔥 将 workflowExecService 注入到 taskService 中，用于自动任务流转功能 ⭐
	// 注意：必须在 workflowExecService 创建后才能注入，避免 nil pointer
	if ts, ok := taskService.(*services.TaskService); ok {
		ts.SetWorkflowExecuteService(workflowExecService)
	}

	// 🔥 将 workflowExecService 注入到 websocketService 中，用于 Worker 回写状态时触发任务流转 ⭐
	// 注意：必须在 workflowExecService 创建后才能注入，避免 nil pointer
	if ws, ok := websocketService.(*services.WebsocketService); ok {
		ws.SetWorkflowExecuteService(workflowExecService)
	}

	// 🔥 将 workflowExecService 注入到 dispatchService 中，用于超时任务触发任务流转 ⭐
	// 注意：必须在 workflowExecService 创建后才能注入，避免 nil pointer
	if ds, ok := dispatchService.(*services.DispatchService); ok {
		ds.SetWorkflowExecuteService(workflowExecService)
	}

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
		taskRoutes.POST("/:id/retry/", taskController.Retry)               // 🔥 手动重试失败的任务
		taskRoutes.POST("/:id/cancel/", taskController.Cancel)             // 🔥 取消待执行任务
		taskRoutes.POST("/:id/stop/", taskController.StopTask)             // 🔥 停止/强制终止正在运行的任务（通过force参数控制）
	}

	// ========== 统计分析模块 ==========
	// 提供深度数据分析和趋势统计，专注于任务执行效率和系统稳定性
	// 🔥 P2架构优化：使用分层架构（Controller -> Service -> Store -> Database）
	// 🔥 P4架构优化：队列健康度使用内存缓存（后台30秒更新）
	statsStore := store.NewStatsStore(db)
	statsService := services.NewStatsService(statsStore)

	// 🔥 创建队列健康度指标管理器（内存缓存 + 后台更新）
	// 需要在 dispatch() 中启动后台更新任务
	var queueMetrics *services.QueueMetrics
	queueMetrics = services.NewQueueMetrics(taskService)

	statsAnalysisController := controllers.NewStatsAnalysisController(taskService, statsService, queueMetrics)
	apis.GET("/task/analysis/", authGroup.Standard, statsAnalysisController.GetAnalysis) // 获取统计分析

	// ========== 统计数据聚合模块 ==========
	// 提供手动触发统计数据聚合的 API，用于服务挂掉后的数据补偿
	// 🔥 使用分布式锁防止并发执行，需要管理员权限
	// 🔥 架构层次：Controller -> Service -> Store -> Database
	statsAggregatorStore := store.NewStatsAggregatorStore(db)
	statsAggregator := services.NewStatsAggregator(statsAggregatorStore)
	statsAggregatorController := controllers.NewStatsAggregatorController(statsAggregator, lockerService)
	apis.POST("/stats/aggregate/daily", authGroup.Admin, statsAggregatorController.TriggerDailyAggregation)           // 手动触发每日聚合
	apis.POST("/stats/aggregate/historical", authGroup.Admin, statsAggregatorController.TriggerHistoricalAggregation) // 手动触发历史数据聚合

	// ========== 任务日志管理模块 ==========
	// 管理任务执行的详细日志，需要用户认证
	// 🔥 使用分片感知的TaskLog服务，支持按月分片存储
	shardConfig := &shard.ShardConfig{
		TablePrefix:    "task_logs",
		ShardBy:        "created_at",
		ShardUnit:      "month",
		AutoCreateNext: true,
		CheckInterval:  "24h",
	}
	shardManager := shard.NewShardManager(db, shardConfig)
	taskLogShardStore := store.NewTaskLogShardStore(db, shardManager)
	taskLogService := services.NewTaskLogShardService(taskLogShardStore)
	taskLogController := controllers.NewTaskLogController(taskLogService, taskService) // 🔥 P2优化：注入taskService用于自动获取created_at

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
	// 提供与Worker节点的实时通信能力，现在使用分布式锁进行安全验证
	// WebSocket连接需要先获取锁令牌，然后验证锁的有效性
	// 注意：websocketService已在Task模块创建（第198行），此处直接使用
	websocketController := controllers.NewWebsocketController(websocketService, lockerService)

	// WebSocket连接接口，不使用认证中间件（有自己的认证机制）
	wsRoutes := apis.Group("/ws")
	{
		wsRoutes.GET("/task/", websocketController.HandleConnect) // WebSocket连接
	}

	// ========== 系统健康检查模块 ==========
	// 系统健康检查，不需要认证（公共接口）
	healthController := controllers.NewHealthController(websocketService, taskService)

	// 健康检查路由（无需认证）
	app.GET("/health", healthController.Health)       // 详细健康检查
	app.GET("/readiness", healthController.Readiness) // 就绪检查（K8s readiness probe）
	app.GET("/liveness", healthController.Liveness)   // 存活检查（K8s liveness probe）

	// 兼容原有的API路径
	healthRoutes := apis.Group("/health")
	{
		healthRoutes.GET("/", healthController.Health) // 系统健康检查（兼容）
	}

	// ========== 监控指标模块 ==========
	// Prometheus监控指标端点，不需要认证（但可以考虑在生产环境中保护）
	metricsController := controllers.NewMetricsController()

	// 监控指标直接注册到app根路由，不经过apis路由组，避免中间件影响
	app.GET("/metrics", metricsController.Metrics)

	// ========== 审批管理模块 ==========

	// ========== Usercenter服务 ==========
	// 🔥 创建 Usercenter Service（用于发送站内信通知）
	// 复用 Auth 配置（Auth 服务就是 Usercenter 服务）
	usercenterService := services.NewUsercenterService(
		config.Auth.ApiUrl,
		config.Auth.ApiKey,
		config.Auth.Timeout,
	)

	// 🔥 审批管理（approvalStore 和 approvalRecordStore 已在前面创建）
	approvalService := services.NewApprovalService(
		approvalStore,
		approvalRecordStore,
		taskStore,
		workflowExecStore,
		workflowExecService, // 🔥 传递 workflowExecService
		usercenterService,   // 🔥 传递 usercenterService
	)
	approvalController := controllers.NewApprovalController(approvalService)

	approvalRoutes := apis.Group("/approvals")
	approvalRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		approvalRoutes.POST("/", approvalController.Create) // 创建审批
		approvalRoutes.GET("/", approvalController.List)    // 获取审批列表

		// 我的审批（必须在 /:id/ 之前注册，避免路由冲突）
		approvalRoutes.GET("/my/pending/", approvalController.MyPending) // 我的待审批
		approvalRoutes.GET("/my/created/", approvalController.MyCreated) // 我发起的审批

		// 单个审批操作（动态路由放在最后）
		approvalRoutes.GET("/:id/", approvalController.Get)                  // 获取单个审批
		approvalRoutes.POST("/:id/action/", approvalController.HandleAction) // 统一审批操作接口（approve/reject/cancel）
		approvalRoutes.DELETE("/:id/", approvalController.Delete)            // 删除审批
	}

	// 审批记录管理
	approvalRecordController := controllers.NewApprovalRecordController(approvalRecordStore)
	approvalRecordRoutes := apis.Group("/approval-records")
	approvalRecordRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		approvalRecordRoutes.GET("/", approvalRecordController.List) // 获取审批记录列表（支持按approval_id过滤）
	}

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

	// 🔥 返回队列健康度指标管理器（需要在 dispatch() 中启动）
	return queueMetrics
}
