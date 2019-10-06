package app

import (
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

type Worker struct {
	TimeStart  time.Time          // 启动时间
	JobManager *common.JobManager // 计划任务管理器
}

func (w *Worker) Run() {
	// 启动worker程序
	log.Println("worker run ...")
	var jobsKeyDir = "/crontab/jobs/"
	var handerWatchDemo = common.WatchHandlerDemo{
		KeyDir: jobsKeyDir,
	}
	// 开始监听keys
	w.JobManager.WatchKeys(jobsKeyDir, &handerWatchDemo)
}

// 实例化Worker
func NewWorker() *Worker {
	var (
		jobManager *common.JobManager
		err        error
	)

	// 实例化jobManager
	if jobManager, err = common.NewJobManager(); err != nil {
		log.Println(err.Error())
		panic(err)
	}

	// 实例化Worker
	return &Worker{
		TimeStart:  time.Now(),
		JobManager: jobManager,
	}
}
