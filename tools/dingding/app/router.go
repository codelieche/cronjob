package app

import (
	"cronjob.codelieche/tools/dingding/handlers"
	"cronjob.codelieche/tools/dingding/middlewares"
	"github.com/kataras/iris"
)

// 设置app的router

func setAppRouter(app *iris.Application) {

	// 使用中间件
	app.Use(middlewares.PrintRequestUrl)

	app.Get("/", handlers.IndexPageWithBasicAuth)

	// 用户相关api
	app.Get("/api/v1/user/list/{page:int min(1)}", handlers.UserListApi)
	app.Get("/api/v1/department/list/{page:int min(1)}", handlers.DepartmentListApi)
}
