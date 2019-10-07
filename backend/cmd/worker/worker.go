package main

import (
	"log"
	"os"
	"runtime"

	"github.com/codelieche/cronjob/backend/worker"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Lshortfile)
}

func main() {
	log.Printf("worker开始运行！进程ID：%d, 父进程ID:%d", os.Getpid(), os.Getppid())

	// 实例化worker
	workerApp := worker.NewWorkerApp()

	// 运行worker程序
	workerApp.Run()
}
