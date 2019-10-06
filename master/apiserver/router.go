package apiserver

import (
	"net/http"

	"cronjob.codelieche/master/handlers"
	"github.com/julienschmidt/httprouter"
)

func newApiRouter() *httprouter.Router {
	router := httprouter.New()

	// 开始添加路由等
	// Pages
	router.GET("/", handlers.IndexPage)

	router.GET("/demo/", handlers.IndexDemo)
	router.GET("/demo/hello/:name", handlers.HelloDemo)

	// job相关
	// job create
	router.POST("/job/create", handlers.JobCreate)
	// job List
	router.GET("/job/list", handlers.JobList)
	// job Detail
	router.GET("/job/detail/:name", handlers.JobDetail)
	// job Delete
	router.DELETE("/job/:name", handlers.JobDelete)

	// job kill相关
	router.POST("/job/kill/create", handlers.JobKill)

	// 静态文件相关： 需要以相对路径
	//router.ServeFiles("/*filepath", http.Dir("./static"))
	router.ServeFiles("/static/*filepath", http.Dir("./static"))
	return router
}
