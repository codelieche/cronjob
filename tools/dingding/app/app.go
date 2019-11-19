package app

import (
	"fmt"
	"log"

	"github.com/codelieche/cronjob/tools/dingding/common"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/logger"
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

	// 使用中间件：添加loger
	//app.Use(recover.New())
	app.Use(logger.New())

	// 处理错误页面
	handleAppOnError(app)

	// 设置View路径
	app.RegisterView(iris.HTML("./web/templates/", ".html"))

	// 当执行kill的时候执行操作：关闭数据库啊等
	iris.RegisterOnInterrupt(handleAppInterupt)

	// 设置路由：重点
	setAppRouter(app)
	// app.Get("/", handlers.IndexPage)

	return app
}

func Run() {
	app := newApp()

	config := common.GetConfig()
	addr := fmt.Sprintf("%s:%d", config.Http.Host, config.Http.Port)

	// 运行程序
	app.Run(iris.Addr(addr), iris.WithoutServerError(iris.ErrServerClosed))
}
