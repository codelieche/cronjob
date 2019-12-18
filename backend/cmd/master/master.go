package main

import (
	"log"

	"github.com/codelieche/cronjob/backend/master/app"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	log.Println("master开始运行！")

	//// 实例化master app
	//app := master.NewMasterApp()
	//
	//// 运行master app程序
	//app.Run()

	app.Run()
}
