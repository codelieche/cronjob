package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type pageData struct {
	Title   string
	Content string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {

	// 打印程序所在的路径
	if dir, err := filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
		log.Panic(err)
	} else {
		log.Println(dir)
	}
	log.Println(r.URL)

	// 这里要注意templates的路径问题：
	// 测试的时候可把templates/下的文件复制到：${GOPATH}/src/templates/中
	GOPATH := os.Getenv("GOPATH")

	templateFilePath := fmt.Sprintf("%s/src/templates/%s", GOPATH, "base.html")
	files := []string{templateFilePath}
	templates := template.Must(template.ParseFiles(files...))

	//log.Println(templates)
	data := &pageData{
		Title:   "Good",
		Content: "This is for test page",
	}

	// 接收传递的值，渲染到页面中
	r.ParseForm()
	title := r.FormValue("title")
	if title != "" {
		data.Title = title
	}

	content := r.FormValue("content")
	if content != "" {
		data.Content = content
	}

	// 渲染模板
	templates.Execute(w, data)

}

func main() {
	http.HandleFunc("/", handleIndex)

	server := &http.Server{
		Addr: ":9090",
	}

	//	启动服务
	server.ListenAndServe()
}
