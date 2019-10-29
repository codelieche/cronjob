package apiserver

import (
	"net/http"

	"github.com/codelieche/cronjob/backend/master/handlers"
	"github.com/julienschmidt/httprouter"
)

func newApiRouter() *httprouter.Router {
	router := httprouter.New()

	// 开始添加路由等
	// Pages
	router.GET("/", handlers.IndexPage)

	router.GET("/demo/", handlers.IndexDemo)
	router.GET("/demo/hello/:name", handlers.HelloDemo)

	// Worker相关
	router.GET("/worker/list", handlers.WorkerList)

	// category相关
	router.GET("/categories/list", handlers.CategoriesList)
	router.POST("/category/create", handlers.CategoryCreate)
	router.GET("/category/:name", handlers.CategoryDetail)
	router.PUT("/category/:name", handlers.CategoryUpdate)

	// job相关
	// job create
	router.POST("/job/create", handlers.JobCreate)
	// job List
	router.GET("/job/list", handlers.JobList)
	// job Detail
	router.GET("/job/detail/:category/:name", handlers.JobDetail)
	// job Update
	router.PUT("/job/:category/:name", handlers.JobUpdate)
	// job Delete
	router.DELETE("/job/:category/:name", handlers.JobDelete)

	// job kill相关
	router.POST("/job/kill/create", handlers.JobKill)

	// 执行日志相关
	router.GET("/job/execute/list", handlers.LogList)
	router.GET("/job/execute/list/:page", handlers.LogList)

	// 静态文件相关： 需要以相对路径
	//router.ServeFiles("/*filepath", http.Dir("./static"))
	router.ServeFiles("/static/*filepath", http.Dir("./static"))

	return router
}
