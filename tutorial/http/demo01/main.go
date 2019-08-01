package main

import (
	"log"
	"net/http"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	w.Write([]byte("Hello Index!"))
	return
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	w.Write([]byte("Hello World!"))
	return
}

func main() {
	log.Println("程序开始")

	//	实例化web
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/hello", handleHello)

	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Panic(err)
	}
}
