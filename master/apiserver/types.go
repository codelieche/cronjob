package apiserver

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type ApiServer struct {
	Router *httprouter.Router
}

// 启动apiServer的http服务
func (apiServer *ApiServer) Run(addr string) {
	//	启动apiServer的http服务器
	if addr == "" {
		addr = ":9000"
	}

	http.ListenAndServe(addr, apiServer.Router)
}
