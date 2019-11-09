package app

import (
	"log"

	"github.com/kataras/iris"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func newApp() *iris.Application {
	app := iris.New()

	// 配置应用
	appConfigure(app)

	// 设置auth
	appAddBasictAuth(app)

	// 处理错误页面
	handleAppOnError(app)

	// 设置View路径
	app.RegisterView(iris.HTML("./templates/", ".html"))

	// 当执行kill的时候执行操作：关闭数据库啊等
	iris.RegisterOnInterrupt(handleAppInterupt)

	// 设置路由：重点
	setAppRouter(app)
	// app.Get("/", handlers.IndexPage)

	return app
}

func Run() {
	app := newApp()

	// 运行程序
	app.Run(iris.Addr(":9000"), iris.WithoutServerError(iris.ErrServerClosed))
}
