package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Index Page
// 网站首页
func IndexPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 返回个静态页面
	http.ServeFile(w, r, "./templates/index.html")
	return
}
