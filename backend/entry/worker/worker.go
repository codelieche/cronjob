package main

import (
	"log"

	//_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/codelieche/cronjob/backend/worker"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Lshortfile)
}

func main() {

	// 性能测试
	// go tool pprof http://localhost:9099/debug/pprof/profile
	// http://localhost:9099/debug/pprof/
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:9099", nil))
	//}()

	log.Printf("worker开始运行！进程ID：%d, 父进程ID:%d", os.Getpid(), os.Getppid())

	// 实例化worker
	workerApp := worker.NewWorkerApp()

	// 运行worker程序
	workerApp.Run()
}
