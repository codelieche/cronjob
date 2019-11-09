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
}
