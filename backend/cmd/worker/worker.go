package main

import (
	"log"
	"os"
	"runtime"

	"github.com/codelieche/cronjob/backend/worker/app"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	log.Printf("worker开始运行！进程ID：%d, 父进程ID:%d", os.Getpid(), os.Getppid())

	// 实例化worker
	worker := app.NewWorker()

	// 运行worker程序
	worker.Run()
}
