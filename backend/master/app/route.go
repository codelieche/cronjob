package app

import (
	"time"

	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/codelieche/cronjob/backend/common/repositories"
	"github.com/codelieche/cronjob/backend/master/web/services"

	"github.com/codelieche/cronjob/backend/master/web/controllers"

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
	mvc.Configure(apiV1.Party("/category"), func(app *mvc.Application) {
		// 实例化category的Repository
		repo := repositories.NewCategoryRepository(db)
		// 实例化category的Service
		service := services.NewCategoryService(repo)
		// 注册service
		app.Register(service)
		// 添加Controller
		app.Handle(new(controllers.CategoryController))
	})
}
