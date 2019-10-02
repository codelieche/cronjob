package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func PageIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "Hello Index Page!\n")
}

func PageHello(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fmt.Fprintf(w, "Hello %s!\n", ps.ByName("name"))
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	log.Println("程序开始运行！")

	router := httprouter.New()
	router.GET("/", PageIndex)
	router.GET("/hello/:name", PageHello)

	log.Println("开始启动http Server")
	http.ListenAndServe(":9000", router)
}
