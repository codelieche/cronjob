package main

import (
	"fmt"
	"net/http"
)

// 处理器函数：index
func handleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello Index! %s", r.URL.Path)
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

	mux.HandleFunc("/", handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", staticFilesHandle))
	mux.HandleFunc("/hello", handleHello)

	//	实例化http Server
	server := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	//	启动服务
	server.ListenAndServe()
}
