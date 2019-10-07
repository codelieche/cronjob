package app

import (
	"log"
	"time"

	"cronjob.codelieche/backend/worker"

	"github.com/codelieche/cronjob/backend/common"
)

type Worker struct {
	TimeStart  time.Time          // 启动时间
	JobManager *common.JobManager // 计划任务管理器
	Scheduler  *worker.Scheduler  // 调度器
}

func (w *Worker) Run() {
	// 启动worker程序
	log.Println("worker run ...")
	var jobsKeyDir = "/crontab/jobs/"
	//var handerWatchDemo = common.WatchHandlerDemo{
	//	KeyDir: jobsKeyDir,
	//}

	var watchHandler = worker.WatchHandler{
		KeyDir:    jobsKeyDir,
		Scheduler: w.Scheduler,
	}

	// 开始监听keys
	//go w.JobManager.WatchKeys(jobsKeyDir, &handerWatchDemo)
	go w.JobManager.WatchKeys(jobsKeyDir, &watchHandler)

	w.Scheduler.ScheduleLoop()
}

// 实例化Worker
func NewWorker() *Worker {
	var (
		jobManager *common.JobManager
		scheduler  *worker.Scheduler
		err        error
	)

	// 实例化jobManager
	if jobManager, err = common.NewJobManager(); err != nil {
		log.Println(err.Error())
		panic(err)
	}

	// 实例化调度器
	scheduler = worker.NewScheduler()

	// 实例化Worker
	return &Worker{
		TimeStart:  time.Now(),
		JobManager: jobManager,
		Scheduler:  scheduler,
	}
}
