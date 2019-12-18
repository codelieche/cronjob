package main

import (
	"fmt"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
)

func main() {
	app := iris.New()
	app.Logger().SetLevel("debug")
	app.Use(recover.New())
	app.Use(logger.New())

	// Index Page
	app.Handle("GET", "/", func(ctx iris.Context) {
		ctx.HTML("Hello Index")
	})

	// Ping Page
	app.Handle("GET", "/ping", func(ctx iris.Context) {
		ctx.Text("pong")
	})

	// User Page By ID
	app.Handle("GET", "/user/{id:int}", func(ctx iris.Context) {
		id := ctx.Params().GetIntDefault("id", 0)
		msg := fmt.Sprintf("User %d Page, By ID", id)
		ctx.Text(msg)
	})

	// User Page By Name
	app.Handle("GET", "/user/{name:string}/info", func(ctx iris.Context) {
		name := ctx.Params().GetStringDefault("name", "No Name")
		msg := fmt.Sprintf("User %s Page, By Name!", name)
		ctx.Text(msg)
	})

	// Run
	app.Run(iris.Addr(":9090"), iris.WithoutServerError(iris.ErrServerClosed))
}
