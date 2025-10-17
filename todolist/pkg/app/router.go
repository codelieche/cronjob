// Package app 应用程序核心模块
//
// 负责应用程序的初始化、配置和启动流程
// 包括路由初始化、后台服务启动等核心功能
package app

import (
	"net/http"

	_ "github.com/codelieche/todolist/docs" // 导入生成的 Swagger 文档
	"github.com/codelieche/todolist/pkg/config"
	"github.com/codelieche/todolist/pkg/controllers"
	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/middleware"
	"github.com/codelieche/todolist/pkg/services"
	"github.com/codelieche/todolist/pkg/store"
	"github.com/codelieche/todolist/pkg/utils/logger"
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
//   - 待办事项管理 (/api/v1/todolist/)
//   - 健康检查 (/api/v1/health/)
//
// 参数:
//   - app: Gin引擎实例，用于注册路由
func initRouter(app *gin.Engine) {
	// 根路径 - 系统状态检查
	app.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "TodoList API Server 运行正常",
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
	sstore := cookie.NewStore([]byte(config.Web.SessionSecretKey))

	// 配置Session选项
	sstore.Options(sessions.Options{
		Secure:   false,         // 开发环境可以使用HTTP，生产环境应设为true
		SameSite: 5,             // SameSite=Lax，防止CSRF攻击
		Path:     "/",           // 所有路径都可用
		MaxAge:   3600 * 24 * 7, // 7天过期
	})

	// 为API路由组添加Session中间件
	apis.Use(sessions.Sessions(config.Web.SessionIDName, sstore))

	// ========== 创建认证中间件组合 ==========
	// 使用新的模块化认证中间件设计
	authGroup := middleware.NewAuthMiddlewareGroup()

	// ========== 待办事项管理模块 ==========
	// 核心模块：管理用户的待办事项，需要用户认证
	todoStore := store.NewTodoListStore(db)
	todoService := services.NewTodoListService(todoStore)
	todoController := controllers.NewTodoListController(todoService)

	// 待办事项管理接口需要用户认证
	todoRoutes := apis.Group("/todolist")
	todoRoutes.Use(authGroup.Standard) // 使用标准认证中间件
	{
		todoRoutes.POST("/", todoController.Create)                                         // 创建待办事项
		todoRoutes.GET("/", todoController.List)                                            // 获取待办事项列表
		todoRoutes.GET("/stats/", todoController.GetStats)                                  // 获取统计信息
		todoRoutes.GET("/calendar/", todoController.GetByTimeRange)                         // 🔥🔥 获取时间区间内的待办事项（日历视图专用，OR 逻辑）
		todoRoutes.GET("/:id/", todoController.Find)                                        // 根据ID获取待办事项
		todoRoutes.PUT("/:id/", todoController.Update)                                      // 更新待办事项信息
		todoRoutes.DELETE("/:id/", todoController.Delete)                                   // 删除待办事项
		todoRoutes.PATCH("/:id/", todoController.Patch)                                     // 部分更新待办事项
		todoRoutes.PUT("/:id/status/", todoController.UpdateStatus)                         // 更新待办事项状态
		todoRoutes.PUT("/:id/complete-with-children/", todoController.MarkDoneWithChildren) // 🔥 批量完成任务及其所有子任务
	}

	// ========== 统计分析模块 ==========
	// 提供深度数据分析和趋势统计
	statsAnalysisController := controllers.NewStatsAnalysisController(todoService)
	apis.GET("/todolist/analysis/", authGroup.Standard, statsAnalysisController.GetAnalysis) // 获取统计分析

	// ========== 系统健康检查模块 ==========
	// 系统健康检查，不需要认证（公共接口）
	healthController := controllers.NewHealthController(todoService)

	// 健康检查路由（无需认证）
	app.GET("/health", healthController.Health)       // 详细健康检查
	app.GET("/readiness", healthController.Readiness) // 就绪检查（K8s readiness probe）
	app.GET("/liveness", healthController.Liveness)   // 存活检查（K8s liveness probe）

	// 兼容原有的API路径
	healthRoutes := apis.Group("/health")
	{
		healthRoutes.GET("/", healthController.Health) // 系统健康检查（兼容）
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
}
