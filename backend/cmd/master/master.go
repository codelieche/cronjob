package main

import (
	"log"

	"cronjob.codelieche/backend/master/app"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	log.Println("master开始运行！")

	// 实例化master
	master := app.NewMasterApp()

	// 运行master程序
	master.Run()
}
