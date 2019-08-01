package main

import (
	"fmt"
	"net/http"
)

//  处理器函数：实际上是一个接受http.ResponseWrite,*http.Request参数的Go函数
//  处理器：是一个拥有：ServerHTTP方法的接口，ServerHTTP接收2个参数，与处理器函数的参数相同
//  处理器的接口：ServeHTTP(ResponseWriter, *Request)

//  自定义处理器:index
type indexHandler struct {
}

func (h *indexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Index Page! %s", r.URL.Path)
}

//  自定义处理器:hello
type helloHandler struct {
}

func (h *helloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello Page! %s", r.URL.Path)
}

func main() {
	index := indexHandler{}
	hello := helloHandler{}

	http.Handle("/", &index)
	http.Handle("/hello/", &hello)

	// 实例化server
	server := &http.Server{
		Addr: ":9090",
	}

	// 启动server
	server.ListenAndServe()

}
