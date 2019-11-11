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

	// 同步钉钉数据
	app.Get("/api/v1/dingding/rsync", handlers.RsyncDingdingData)

	// 用户相关api
	app.Get("/api/v1/user/{id:string}", handlers.GetUserDetail)
	app.Get("/api/v1/user/list/{page:int min(1)}", handlers.UserListApi)

	// 部门相关api
	app.Get("/api/v1/department/{name:string}", handlers.GetDepartmentDetail)
	app.Get("/api/v1/department/list/{page:int min(1)}", handlers.DepartmentListApi)

	//	发送消息
	app.Post("/api/v1/message/create", handlers.SendWorkerMessageToUser)
	//	消息列表
	app.Get("/api/v1/message/list/{page:int min(1)}", handlers.MessageListApi)
	// 消息详情
	app.Get("/api/v1/message/{id:int min(1)}", handlers.GetMessageDetailApi)
}
