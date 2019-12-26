package app

import (
	"time"

	"github.com/codelieche/cronjob/backend/master/sockets"

	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/codelieche/cronjob/backend/common/repositories"
	"github.com/codelieche/cronjob/backend/master/web/controllers"
	"github.com/codelieche/cronjob/backend/master/web/services"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
)

func setAppRoute(app *iris.Application) {
	// 首页
	mvc.Configure(app.Party("/"), func(app *mvc.Application) {
		// session
		// 注册控制器需要Session和StartTime
		app.Register(
			sess.Start,
			time.Now(),
		)
		app.Handle(new(controllers.IndexController))
	})

	// /api/v1相关的路由
	apiV1 := app.Party("/api/v1")
	// /api/v1开头的url都需要使用IsAuthenticatedMiddleware的中间件
	// apiV1.Use(middlewares.IsAuthenticatedMiddleware)

	// 分类相关的api
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()
	mvc.Configure(apiV1.Party("/category"), func(app *mvc.Application) {
		// 实例化category的Repository
		repo := repositories.NewCategoryRepository(db, etcd)
		// 实例化category的Service
		service := services.NewCategoryService(repo)
		// 注册service
		app.Register(service, sess.Start)
		// 添加Controller
		app.Handle(new(controllers.CategoryController))
	})

	// Job相关的api
	mvc.Configure(apiV1.Party("/job"), func(app *mvc.Application) {
		// 实例化Job的repository
		repo := repositories.NewJobRepository(db, etcd)
		// 实例化Job的Service
		service := services.NewJobService(repo)
		// 注册Service
		app.Register(service, sess.Start)
		// 添加Controller
		app.Handle(new(controllers.JobController))
	})

	// Job Kill相关的api
	mvc.Configure(apiV1.Party("/job/kill"), func(app *mvc.Application) {
		// 实例化JobKill的repository
		repo := repositories.NewJobKillRepository(db, etcd)
		// 实例化Job的Service
		service := services.NewJobKillService(repo)
		// 注册Service
		app.Register(service, sess.Start)
		// 添加Controller
		app.Handle(new(controllers.JobKillController))
	})

	// JobExecute相关的api
	mvc.Configure(apiV1.Party("/job/execute"), func(app *mvc.Application) {
		// 实例化JobExecute的repository
		mongoDB := datasources.GetMongoDB()
		//etcd := datasources.GetEtcd()
		repo := repositories.NewJobExecuteRepository(db, etcd, mongoDB)
		// 实例化JobExecute的Service
		service := services.NewJobExecuteService(repo)
		// 注册Service
		app.Register(service, sess.Start)
		// 添加Controller
		app.Handle(new(controllers.JobExecuteController))
	})

	// Worker相关的api
	mvc.Configure(apiV1.Party("/worker"), func(app *mvc.Application) {
		// 实例化Worker的repository
		etcd := datasources.GetEtcd()
		repo := repositories.NewWorkerRepository(etcd)
		// 实例化Worker的Service
		service := services.NewWorkerService(repo)
		// 注册Service
		app.Register(service, sess.Start)
		// 添加Controller
		app.Handle(new(controllers.WorkerController))
	})

	// 处理websocket
	mvc.Configure(app.Party("/websocket"), func(app *mvc.Application) {
		// 注册
		app.Register(sess.Start)
		// 添加Controller
		app.Handle(new(sockets.WebsocketController))
	})
}
