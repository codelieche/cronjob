package app

import (
	"log"

	"github.com/kataras/iris"
)

// 设置app的错误页面
func handleAppOnError(app *iris.Application) *iris.Application {
	// 404错误
	app.OnErrorCode(iris.StatusNotFound, notFound)
	// 500错误
	app.OnErrorCode(iris.StatusInternalServerError, internalServerError)
	return app
}

func notFound(ctx iris.Context) {
	log.Println(ctx.Method(), ctx.Request().URL, ctx.GetStatusCode())
	ctx.ViewData("url", ctx.Request().URL)
	ctx.View("errors/404.html")
}

func internalServerError(ctx iris.Context) {
	log.Println(ctx.Method(), ctx.Request().URL, ctx.GetStatusCode())
	ctx.View("errors/500.html")
}
