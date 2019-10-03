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

	return router
}
