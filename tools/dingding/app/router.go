package app

import (
	"time"

	"github.com/codelieche/cronjob/tools/dingding/base/handlers"
	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datasource"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
	"github.com/codelieche/cronjob/tools/dingding/web/controllers"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
	"github.com/kataras/iris"
	"github.com/kataras/iris/mvc"
	"github.com/kataras/iris/sessions"
)

// 设置app的router
func setAppRouter(app *iris.Application) {

	// 使用中间件
	// app.Use(logger.New())

	mvc.Configure(app.Party("/"), func(app *mvc.Application) {
		// session
		session := sessions.New(sessions.Config{Cookie: "session_cookie_name"})
		// 注册控制器需要的Session和StartTime
		app.Register(
			session.Start,
			time.Now(),
		)
		app.Handle(new(controllers.IndexController))
	})

	// /api/v1 相关的api
	apiV1 := app.Party("/api/v1")
	// 部门相关api
	mvc.Configure(apiV1.Party("/department"), func(app *mvc.Application) {
		// 实例化User的Repository
		repo := repositories.NewDepartmentRepository(datasource.DB)
		// 实例化User的Service
		deptmentService := services.NewDepartmentService(repo)
		// 注册Service
		app.Register(deptmentService)
		// 添加Crontroller
		app.Handle(new(controllers.DepartmentController))
	})

	// 用户相关api
	mvc.Configure(apiV1.Party("/user"), func(app *mvc.Application) {
		// 实例化User的Repository
		repo := repositories.NewUserRepository(datasource.DB)
		// 实例化User的Service
		userService := services.NewUserService(repo)
		// 注册Service
		app.Register(userService)
		// 添加Crontroller
		app.Handle(new(controllers.UserController))
	})

	// 消息相关api
	mvc.Configure(apiV1.Party("/message"), func(app *mvc.Application) {
		ding := common.NewDing()
		// 实例化User的Repository
		repo := repositories.NewMessageRepository(datasource.DB, ding)
		userRepo := repositories.NewUserRepository(datasource.DB)

		// 实例化Message的Service
		messageService := services.NewMessageService(repo, userRepo)

		// 注册Service
		app.Register(messageService)
		// 添加Crontroller
		app.Handle(new(controllers.MessageController))
	})

	// 同步钉钉数据
	mvc.Configure(apiV1.Party("/dingding/rsync"), func(app *mvc.Application) {
		app.Handle(new(controllers.RsyncController))
	})

	// 测试MVC
	mvc.Configure(apiV1.Party("/movies"), func(app *mvc.Application) {
		repo := repositories.NewMovieRepository(datasource.Movies)
		movieService := services.NewMoviewService(repo)
		app.Register(movieService)

		app.Handle(new(controllers.MovieController))
	})
}

// 老的版本API
func setAppRouterV0(app *iris.Application) {
	app.Get("/v0/", handlers.IndexPageWithBasicAuth)

	// 同步钉钉数据
	app.Get("/api/v0/dingding/rsync", handlers.RsyncDingdingData)

	// 用户相关api
	app.Get("/api/v0/user/{id:string}", handlers.GetUserDetail)
	app.Get("/api/v0/user/list/{page:int min(1)}", handlers.UserListApi)
	// 获取用户消息列表
	app.Get("/api/v0/user/{id:string}/message/list", handlers.GetUserMessageListApi)

	// 部门相关api
	app.Get("/api/v0/department/{name:string}", handlers.GetDepartmentDetail)
	app.Get("/api/v0/department/list/{page:int min(1)}", handlers.DepartmentListApi)

	//	发送消息
	app.Post("/api/v0/message/create", handlers.SendWorkerMessageToUser)
	//	消息列表
	app.Get("/api/v0/message/list/{page:int min(1)}", handlers.MessageListApi)
	// 消息详情
	app.Get("/api/v0/message/{id:int min(1)}", handlers.GetMessageDetailApi)
}
