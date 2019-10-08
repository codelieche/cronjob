package worker

// 计划任务的执行器
var executor *Executor
var app *Worker

func init() {
	executor = NewExecutor()
	app = NewWorkerApp()

}
