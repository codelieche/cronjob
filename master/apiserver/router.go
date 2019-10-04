package apiserver

import (
	"cronjob.codelieche/master/handlers"
	"github.com/julienschmidt/httprouter"
)

func newApiRouter() *httprouter.Router {
	router := httprouter.New()

	// 开始添加路由等
	router.GET("/", handlers.IndexDemo)
	router.GET("/hello/:name", handlers.HelloDemo)

	// job相关
	// job create
	router.POST("/job/create", handlers.JobCreate)
	// job Detail
	router.GET("/job/:name", handlers.JobDetail)
	// job Delete
	router.DELETE("/job/:name", handlers.JobDelete)

	return router
}
