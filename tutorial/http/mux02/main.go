package main

import (
	"fmt"
	"log"
	"net/http"
)

// 传入个处理器函数，然后返回个处理器函数
// 这样的话可用来做中间件
func logHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL)
		h(w, r)
	}
}

// 处理器函数：index
func handleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello  Index! %s", r.URL.Path)
}

// 处理器函数：hello
func handleHello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World! %s", r.URL.Path)
}

func main() {
	// 多路复用器
	mux := http.NewServeMux()

	// 静态文件处理
	staticFilesHandle := http.FileServer(http.Dir("/data/www/static/"))

	mux.HandleFunc("/", logHandler(handleIndex))
	mux.HandleFunc("/hello/", logHandler(handleHello))

	mux.Handle("/static/", http.StripPrefix("/static/", staticFilesHandle))

	//	实例化http Server
	server := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	//	启动服务
	server.ListenAndServe()
}
