package app

import (
	"github.com/codelieche/cronjob/tools/dingding/datasource"
	"github.com/codelieche/cronjob/tools/dingding/handlers"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
	"github.com/codelieche/cronjob/tools/dingding/web/crontollers"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
	"github.com/kataras/iris"
	"github.com/kataras/iris/mvc"
)

// 设置app的router

func setAppRouter(app *iris.Application) {

	// 使用中间件
	// app.Use(logger.New())
	// app.Use(middlewares.PrintRequestUrl) // Demo

	app.Get("/", handlers.IndexPageWithBasicAuth)

	// 同步钉钉数据
	app.Get("/api/v1/dingding/rsync", handlers.RsyncDingdingData)

	// 用户相关api
	app.Get("/api/v1/user/{id:string}", handlers.GetUserDetail)
	app.Get("/api/v1/user/list/{page:int min(1)}", handlers.UserListApi)
	// 获取用户消息列表
	app.Get("/api/v1/user/{id:string}/message/list", handlers.GetUserMessageListApi)

	// 部门相关api
	app.Get("/api/v1/department/{name:string}", handlers.GetDepartmentDetail)
	app.Get("/api/v1/department/list/{page:int min(1)}", handlers.DepartmentListApi)

	//	发送消息
	app.Post("/api/v1/message/create", handlers.SendWorkerMessageToUser)
	//	消息列表
	app.Get("/api/v1/message/list/{page:int min(1)}", handlers.MessageListApi)
	// 消息详情
	app.Get("/api/v1/message/{id:int min(1)}", handlers.GetMessageDetailApi)

	// 测试MVC
	//repo := repositories.NewMovieRepository(datasource.Movies)
	//movieService := services.NewMoviewService(repo)
	//mvc.New(app).Handle(movieService)
	//m := app.Party("/movies")
	//mvc.New(m).Handle(movieService)

	mvc.Configure(app.Party("/movies"), func(app *mvc.Application) {
		repo := repositories.NewMovieRepository(datasource.Movies)
		movieService := services.NewMoviewService(repo)
		app.Register(movieService)

		app.Handle(new(crontollers.MovieController))
	})

}
