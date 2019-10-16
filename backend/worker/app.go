package worker

import (
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

type Worker struct {
	TimeStart  time.Time          // 启动时间
	JobManager *common.JobManager // 计划任务管理器
	Scheduler  *Scheduler         // 调度器
}

func (w *Worker) Run() {
	// 启动worker程序
	log.Println("worker run ...")

	// 启动worker的监控web协程
	go runMonitorWeb()

	//var jobsKeyDir = "/crontab/jobs/"
	var jobsKeyDir = common.ETCD_JOBS_DIR
	//var handerJobsWatchDemo = common.WatchJobsHandlerDemo{
	//	KeyDir: jobsKeyDir,
	//}

	var watchHandler = WatchJobsHandler{
		KeyDir:    jobsKeyDir,
		Scheduler: w.Scheduler,
	}

	// watch kill
	var watchKillHandler = &WatchKillHandler{
		KeyDir:    common.ETCD_JOB_KILL_DIR,
		Scheduler: w.Scheduler,
	}

	// 开始监听keys
	//go w.JobManager.WatchKeys(jobsKeyDir, &handerWatchDemo)
	// 监听jobs
	go w.JobManager.WatchKeys(jobsKeyDir, &watchHandler)
	// 监听kill
	go w.JobManager.WatchKeys(common.ETCD_JOB_KILL_DIR, watchKillHandler)

	// 注册worker信息到etcd
	go register.keepOnlive()

	w.Scheduler.ScheduleLoop()
}

// 实例化Worker
func NewWorkerApp() *Worker {

	// 定义了个全局的app的
	if app != nil {
		return app
	} else {

	}

	var (
		jobManager *common.JobManager
		scheduler  *Scheduler
		err        error
	)

	// 实例化jobManager
	if jobManager, err = common.NewJobManager(common.Config.Worker.Etcd); err != nil {
		log.Println(err.Error())
		panic(err)
	}

	// 实例化调度器
	scheduler = NewScheduler()

	// 实例化Worker
	return &Worker{
		TimeStart:  time.Now(),
		JobManager: jobManager,
		Scheduler:  scheduler,
	}
}
