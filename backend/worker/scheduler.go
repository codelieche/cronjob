package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

// 任务调度器
type Scheduler struct {
	jobEventChan      chan *common.JobEvent              // etcd任务时间队列
	jobPlanTable      map[string]*common.JobSchedulePlan // 任务调度计划表
	jobExecutingTable map[string]*common.JobExecuteInfo  // 任务执行信息表
	jobResultChan     chan *common.JobExecuteResult      // 任务执行结果队列
	logHandler        LogHandler                         // 执行日志处理器
}

// 计算任务调度状态
func (scheduler *Scheduler) TrySchedule() (scheduleAfter time.Duration) {

	var (
		jobPlan  *common.JobSchedulePlan
		now      time.Time
		nearTime *time.Time
		err      error
	)
	// 1. 遍历所有的job

	// 如果任务表为空
	if len(scheduler.jobPlanTable) == 0 {
		scheduleAfter = 1 * time.Second
		return
	}

	// 当前时间
	now = time.Now()
	for _, jobPlan = range scheduler.jobPlanTable {
		// 2. 过期的任务立即执行
		// 如果执行计划下次执行的世界早于当前，或者等于当前时间，都需要执行一下这个计划
		if jobPlan.NextTime.Before(now) || jobPlan.NextTime.Equal(now) {
			// log.Println("执行计划任务：", jobPlan.Job.Name)
			// 执行计划任务
			if err = scheduler.TryRunJob(jobPlan); err != nil {
				log.Println("执行计划任务出错：", err.Error())
			}
			// 更新NextTime：需要设置新的下次执行时间
			jobPlan.NextTime = jobPlan.Expression.Next(now)
		} else {
			//  log.Println(jobPlan.Job.Name, jobPlan.NextTime)
		}

		// 3. 统计最近要过期的任务还需多久
		if nearTime == nil || jobPlan.NextTime.Before(*nearTime) {
			nearTime = &jobPlan.NextTime
		}
	}

	// 4. 返回下次执行TrySchedule的时间
	scheduleAfter = (*nearTime).Sub(now)
	return
}

// 调度协程
func (scheduler *Scheduler) ScheduleLoop() {
	// 1. 定义变量
	var (
		jobEvent      *common.JobEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
	)

	// 首次执行一下：
	scheduleAfter = scheduler.TrySchedule()

	// 调度的延时定时器
	scheduleTimer = time.NewTimer(scheduleAfter)

	// 启动消费执行结果协程
	go scheduler.comsumeJobExecuteResultsLoop()

	// 启动消费执行日志的协程
	go scheduler.logHandler.ConsumeLogsLoop()

	// 2. 定时任务
	for {
		select {
		case jobEvent = <-scheduler.jobEventChan: // 监听任务变化事件
			log.Println("新的jobEvent：", jobEvent.Job.Name, jobEvent.Job.Time, jobEvent.Job.Command)
			// 根据事件，对内存中维护的任务队列做增删操作
			scheduler.handleJobEvent(jobEvent)
		case <-scheduleTimer.C: // Timer到期：最近的任务到期了

		}
		// 再次调度一次任务
		scheduleAfter = scheduler.TrySchedule()
		// 重置一下定时器:
		// 来了新的事件了，或者Timer过期了，都需要重置下Timer
		scheduleTimer.Reset(scheduleAfter)
	}
}

// 推送任务变化事件
func (scheduler *Scheduler) PushJobEvent(jobEvent *common.JobEvent) {
	// 往jobEventChan中投递jobEvent
	scheduler.jobEventChan <- jobEvent
}

// 处理任务事件
func (scheduler *Scheduler) handleJobEvent(jobEvent *common.JobEvent) {
	var (
		jobSchedulePlan *common.JobSchedulePlan
		err             error
		isExist         bool
		jobExecuteInfo  *common.JobExecuteInfo
	)
	switch jobEvent.Event {
	case common.JOB_EVENT_PUT: // 保存job事件
		if jobSchedulePlan, err = common.BuildJobSchedulePlan(jobEvent.Job); err != nil {
			return
		} else {
			scheduler.jobPlanTable[jobEvent.Job.Name] = jobSchedulePlan
		}

	case common.JOB_EVENT_DELETE: // 删除job事件
		// 判断job是否存在
		if jobSchedulePlan, isExist = scheduler.jobPlanTable[jobEvent.Job.Name]; isExist {
			// 存在就删除，不存在就无需操作
			delete(scheduler.jobPlanTable, jobEvent.Job.Name)
		}

	case common.JOB_EVENT_KILL: // 杀掉job事件
		// 取消Command执行
		if jobExecuteInfo, isExist = scheduler.jobExecutingTable[jobEvent.Job.Name]; isExist {
			// 是的在本work中执行中，那么可以杀掉它
			log.Println("需要杀死job")
			jobExecuteInfo.ExceteCancelFun()
		} else {
			log.Println("Job未在执行中，无需kill")
		}

	}
}

// 执行计划任务
func (scheduler *Scheduler) TryRunJob(jobPlan *common.JobSchedulePlan) (err error) {
	var (
		jobExecuteInfo *common.JobExecuteInfo
		isExecuting    bool
	)
	// 如果任务正在执行，跳过本次调度
	if jobExecuteInfo, isExecuting = scheduler.jobExecutingTable[jobPlan.Job.Name]; isExecuting {
		log.Println("尚未退出，还在执行，跳过！", jobPlan.Job.Name)
		return
	} else {
		// 构建执行状态信息
		jobExecuteInfo = common.BuildJobExecuteInfo(jobPlan)

		// 保存执行信息
		scheduler.jobExecutingTable[jobPlan.Job.Name] = jobExecuteInfo

		// 执行计划任务
		executor.ExecuteJob(jobExecuteInfo, scheduler.jobResultChan)
	}

	// 执行完毕后，从执行信息表中删除这条数据,这个在HandlerJobExecuteResult中处理
	// 即使未获取到锁，也需要从scheduler.jobExecutingTable 删除这条jobExecuteInfo

	return
}

// 回传任务执行结果
func (scheduler *Scheduler) PushJobExecuteResult(result *common.JobExecuteResult) {
	scheduler.jobResultChan <- result
}

// 消费计划任务执行结果的循环
func (scheduler *Scheduler) comsumeJobExecuteResultsLoop() {
	var (
		result *common.JobExecuteResult
	)
	for {
		select {
		case result = <-scheduler.jobResultChan:
			scheduler.HandlerJobExecuteResult(result)
		}
	}
}

// 处理计划任务的结果
func (scheduler *Scheduler) HandlerJobExecuteResult(result *common.JobExecuteResult) {
	var (
		jobExecuteLog *common.JobExecuteLog
	)
	// 删掉执行状态
	delete(scheduler.jobExecutingTable, result.ExecuteInfo.Job.Name)
	if result.IsExecute {
		// 记录日志
		jobExecuteLog = &common.JobExecuteLog{
			Name:         result.ExecuteInfo.Job.Name,
			Command:      result.ExecuteInfo.Job.Command,
			Output:       string(result.Output),
			PlanTime:     result.ExecuteInfo.PlanTime,
			ScheduleTime: result.ExecuteInfo.ExecuteTime,
			StartTime:    result.StartTime,
			EndTime:      result.EndTime,
		}

		// 判断是否有错误信息
		if result.Err != nil {
			jobExecuteLog.Err = result.Err.Error()
		}

		// 交给写日志的程序处理。
		scheduler.logHandler.AddLog(jobExecuteLog)

		log.Println("Job执行完成：", result.ExecuteInfo.Job.Name)
		fmt.Println(string(result.Output))
		if result.Err != nil {
			fmt.Println("执行出现了错误：", result.Err.Error())
		}

	} else {
		log.Printf("Job: %s 未执行：%s\n", result.ExecuteInfo.Job.Name, result.Err.Error())
	}
}

// 消费结果

// 初始化调度器
func NewScheduler() *Scheduler {
	var (
		logHandler *MongoLogHandler
		err        error
	)
	// 实例化消息处理
	if logHandler, err = NewMongoLogHandler(); err != nil {
		log.Panic(err)
		return nil
	} else {

	}
	scheduler := &Scheduler{
		jobEventChan:      make(chan *common.JobEvent, 1000),
		jobPlanTable:      make(map[string]*common.JobSchedulePlan),
		jobExecutingTable: make(map[string]*common.JobExecuteInfo),
		jobResultChan:     make(chan *common.JobExecuteResult, 500),
		logHandler:        logHandler,
	}

	return scheduler
}
