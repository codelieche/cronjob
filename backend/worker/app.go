package worker

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

type Worker struct {
	TimeStart  time.Time       // 启动时间
	Scheduler  *Scheduler      // 调度器
	Categories map[string]bool // 执行计划任务的类型
	socket     *Socket         // 工作节点连接的Master socket
	IsActive   bool            // 是否有效
}

func (w *Worker) Run() {
	// 启动worker程序
	log.Println("worker run ...")

	// 捕获退出事件
	catchKillChan := make(chan os.Signal)
	signal.Notify(catchKillChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-catchKillChan
		log.Println("捕获到退出事件")
		// 执行stop相关操作
		w.Stop()
	}()

	// worker初始化：设置工作环境
	config = common.GetConfig().Worker
	// log.Println(config)
	w.setupExecuteEnvrionment()

	// 启动worker的监控web协程
	go runMonitorWeb()

	// 连接master的socket: 回写各种数据，都是通过socket
	connectMasterSocket(1)

	// 注册worker信息到etcd
	//go register.keepOnlive()
	if err := register.postWorkerInfoToMaster(); err != nil {
		log.Println("发送worker信息去master出错", err)
		os.Exit(1)
	}

	w.Scheduler.ScheduleLoop()
}

func (w *Worker) Stop() {
	w.IsActive = false

	// 设置调度为停止
	app.Scheduler.isStoped = true

	// 删除掉worker信息
	register.deleteWorkerInfo()

	// socket发送关闭消息
	w.socket.Stop()

	// 杀掉正在运行的任务
	for k, v := range w.Scheduler.jobExecutingTable {
		log.Println("开始停止：", k)
		// 执行取消函数
		v.ExceteCancelFun()
	}
	// 休眠60秒，等待各任务完全退出。
	time.Sleep(time.Minute)
}

// 实例化Worker
func NewWorkerApp() *Worker {

	// 定义了个全局的app的
	if app != nil {
		return app
	} else {
		var scheduler *Scheduler

		// 实例化调度器
		scheduler = NewScheduler()

		// 实例化Worker
		return &Worker{
			TimeStart:  time.Now(),
			Scheduler:  scheduler,
			Categories: make(map[string]bool),
			IsActive:   true,
		}
	}

}
