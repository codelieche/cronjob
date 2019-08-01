package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

//  cookie的基本使用

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	fmt.Fprintf(w, "Index !%s", r.URL.Path)
}

//  设置cookie
//  需要传递name和value
func handleSetCookie(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	name := r.FormValue("name")
	value := r.FormValue("value")

	//  对值进行检验
	if name == "" || value == "" {
		http.Error(w, "传入的name或者值为空", 400)
		return
	}

	//  设置cookie
	cookie := http.Cookie{
		Name:   name,
		Value:  value,
		MaxAge: 60,
	}

	http.SetCookie(w, &cookie)

	fmt.Fprintf(w, "设置Cookie：%s ===> %s", name, value)

}

//  获取cookie
func handleGetCookie(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	name := r.FormValue("name")
	//  对name进行检查
	if name == "" {
		http.Error(w, "传入的name是空，请重试", 400)
		return
	}

	//  获取cookie
	if c, err := r.Cookie(name); err != nil {
		message := fmt.Sprintf("获取%s的cookie出错：", name, err.Error())
		http.Error(w, message, 400)
		return
	} else {
		value := c.Value
		fmt.Fprintf(w, "获取Cookie：%s ===> %s", name, value)
	}
}

func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/cookie/get", handleGetCookie)
	mux.HandleFunc("/cookie/set", handleSetCookie)

	server := http.Server{
		Addr:              ":9090",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      20 * time.Second,
	}

	//  启动服务
	server.ListenAndServe()

}
