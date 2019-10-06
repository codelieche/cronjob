package main

import (
	"log"
	"runtime"

	"github.com/codelieche/cronjob/backend/worker/app"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	log.Println("worker开始运行！")

	// 实例化worker
	worker := app.NewWorker()

	// 运行worker程序
	worker.Run()
}
