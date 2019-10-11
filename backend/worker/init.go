package worker

import (
	"log"
	"os"
)

// 计划任务的执行器
var executor *Executor
var app *Worker
var register *Register

func init() {
	// 解析参数
	parseParams()

	// 启动worker的监控web协程
	go runMonitorWeb()

	var (
		err error
	)

	executor = NewExecutor()
	app = NewWorkerApp()
	if register, err = newRegister(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}
