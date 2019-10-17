package worker

import (
	"log"
	"os"

	"github.com/codelieche/cronjob/backend/common"
)

// 计划任务的执行器
var executor *Executor
var app *Worker
var register *Register
var config *common.WorkerConfig

func init() {
	var (
		err error
	)

	//webMonitorPort = 8080
	parseParams()

	executor = NewExecutor()
	app = NewWorkerApp()
	if register, err = newRegister(); err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}
