package app

import (
	"fmt"
	"log"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/kataras/iris/v12/middleware/logger"

	"github.com/kataras/iris/v12"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func newApp() *iris.Application {
	app := iris.New()
	// 配置应用
	appConfigure(app)

	// 使用中间件，添加logger
	app.Use(logger.New(logger.Config{
		Status:             true,
		IP:                 true,
		Method:             true,
		Path:               true,
		Query:              true,
		Columns:            false,
		MessageContextKeys: nil,
		MessageHeaderKeys:  nil,
		LogFunc:            nil,
		LogFuncCtx:         nil,
		Skippers:           nil,
	}))

	// 使用session
	useSessionMiddleware(app)

	// 设置View的路径
	viewEngine := iris.HTML("./web/templates", ".html")
	app.RegisterView(viewEngine)

	// 当执行kill的时候执行操作：关闭数据库等
	iris.RegisterOnInterrupt(handleAppOnInterput)

	// 设置路由：重点
	setAppRoute(app)

	// 静态文件
	app.HandleDir("/static", "./web/public")

	// 设置Debug
	app.Logger().SetLevel("debug")

	return app
}

func Run() {
	app := newApp()
	config := common.Config
	addr := fmt.Sprintf("%s:%d", config.Master.Http.Host, config.Master.Http.Port)

	// 运行程序
	app.Run(iris.Addr(addr), iris.WithoutServerError(iris.ErrServerClosed))
}
