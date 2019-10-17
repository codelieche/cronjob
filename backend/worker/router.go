package worker

import "github.com/julienschmidt/httprouter"

// 实例化router
func newWebMonitorRouter() *httprouter.Router {
	router := httprouter.New()

	router.GET("/info", workerInfoHandler)
	router.GET("/categories", categoriesListHandler)
	router.GET("/category/list", categoriesListHandler)
	router.POST("/category/add", categoryAddHandler)
	// 移除worker的category
	router.DELETE("/category/:name", removeCategoryHandler)
	return router
}
