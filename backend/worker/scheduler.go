package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"
)

// 任务调度器
type Scheduler struct {
	jobEventChan      chan *datamodels.JobEvent              // etcd任务时间队列
	jobPlanTable      map[string]*datamodels.JobSchedulePlan // 任务调度计划表
	jobExecutingTable map[string]*datamodels.JobExecuteInfo  // 任务执行信息表
	jobResultChan     chan *datamodels.JobExecuteResult      // 任务执行结果队列
	//logHandler        LogHandler                             // 执行日志处理器
	isStoped bool // 是否停止调度
}

// 计算任务调度状态
// 会尝试执行需要执行的计划任务，并计算jobPlan的下次执行时间
// 计算now与所有jobPlan中最近的下次执行的时间的间隔
func (scheduler *Scheduler) TrySchedule() (scheduleAfter time.Duration) {
	var (
		jobPlan  *datamodels.JobSchedulePlan // 计划任务执行Plan信息
		now      time.Time                   // 当前时间
		nearTime *time.Time                  // 最近一次要执行的计划任务时间
		err      error                       // error
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
		// 当nearTime是空的时候，就赋值当前计划任务的下次执行时间
		// 当当前jobPlan的下次执行时间，早于nearTime就更新一下nearTime
		if nearTime == nil || jobPlan.NextTime.Before(*nearTime) {
			nearTime = &jobPlan.NextTime
		}
	}

	// 4. 返回下次执行TrySchedule的时间
	// 当前时间与最近一次要执行的任务的时间间隔
	scheduleAfter = (*nearTime).Sub(now)
	return
}

// 调度协程
func (scheduler *Scheduler) ScheduleLoop() {
	// 1. 定义变量
	var (
		jobEvent      *datamodels.JobEvent
		scheduleAfter time.Duration
		scheduleTimer *time.Timer
	)

	// 首次执行一下：得到要下次调度的时间间隔
	scheduleAfter = scheduler.TrySchedule()

	// 调度的延时定时器
	scheduleTimer = time.NewTimer(scheduleAfter)

	// 启动消费执行结果协程：里面会用到logHandler
	go scheduler.comsumeJobExecuteResultsLoop()

	// 启动消费执行日志的协程
	//go scheduler.logHandler.ConsumeLogsLoop()

	// 2. 定时任务
	for {
		select {
		case jobEvent = <-scheduler.jobEventChan: // 监听任务变化事件
			// log.Println("新的jobEvent：", jobEvent.Job.Name, jobEvent.Job.Time, jobEvent.Job.Command)
			// 根据事件，对内存中维护的任务队列做增删操作
			scheduler.handleJobEvent(jobEvent)
		case <-scheduleTimer.C: // Timer到期：最近的任务到期了

		}
		// 再次调度一次任务: 执行计划任务是在这里面的
		scheduleAfter = scheduler.TrySchedule()
		// 重置一下定时器:
		// 来了新的事件了，或者Timer过期了，都需要重置下Timer
		scheduleTimer.Reset(scheduleAfter)

		// 判断是否需要跳出调度
		if scheduler.isStoped {
			log.Println("调度器已经是停止了，不再调度任务")
			break
		}
	}

	// 遍历所有执行table设置为kill
	// 手动杀掉所有正在执行的任务
	for _, info := range scheduler.jobExecutingTable {
		info.Status = "kill"
		info.ExceteCancelFun()
	}

	i := 10
	for i > 0 {
		log.Printf("%d秒后程序退出！\n", i)
		time.Sleep(time.Second)
		i--
	}
	log.Println("Done")
}

// 推送任务变化事件
func (scheduler *Scheduler) PushJobEvent(jobEvent *datamodels.JobEvent) {
	// 往jobEventChan中投递jobEvent
	scheduler.jobEventChan <- jobEvent
}

// 处理任务事件
// 在watchHandler中添加jobEvent
// 在ScheduleLoop中消耗jobEvent
// 新增了Job、删除了Job、Job需要Kill的事件
func (scheduler *Scheduler) handleJobEvent(jobEvent *datamodels.JobEvent) {
	var (
		jobSchedulePlan *datamodels.JobSchedulePlan
		jobExecutingKey string // 任务类型 + "-" + 任务名称
		err             error
		isExist         bool
		jobExecuteInfo  *datamodels.JobExecuteInfo
	)
	switch jobEvent.Event {
	case common.JOB_EVENT_PUT: // 保存job事件
		if jobSchedulePlan, err = jobEvent.Job.ToJobExecutePlan(); err != nil {
			log.Println(err)
			return
		} else {
			// 先生成key：根据：分类-名字
			// jobExecutingKey = jobEvent.Job.Category + "-" + jobEvent.Job.Name
			jobExecutingKey = fmt.Sprintf("%s-%d", jobEvent.Job.Category, jobEvent.Job.ID)

			// 判断job是否是激活状态的
			if jobSchedulePlan.Job.IsActive {
				// 加入/修改：jobPlanTable
				scheduler.jobPlanTable[jobExecutingKey] = jobSchedulePlan
			} else {
				// 如果Job存在那么需要删除
				log.Println("当前Job状态是flase，无需添加到执行Table中：", jobSchedulePlan.Job)
				if jobSchedulePlan, isExist = scheduler.jobPlanTable[jobExecutingKey]; isExist {
					// 存在就删除，不存在就无需操作：
					log.Printf("需要把%s从jobPlanTable中删除", jobExecutingKey)
					delete(scheduler.jobPlanTable, jobExecutingKey)
				}
			}
		}

	case common.JOB_EVENT_DELETE: // 删除job事件
		// 判断job是否存在
		//jobExecutingKey = jobEvent.Job.Category + "-" + jobEvent.Job.Name
		jobExecutingKey = fmt.Sprintf("%s-%d", jobEvent.Job.Category, jobEvent.Job.ID)
		if jobSchedulePlan, isExist = scheduler.jobPlanTable[jobExecutingKey]; isExist {
			// 存在就删除，不存在就无需操作
			log.Printf("需要把%s从jobPlanTable中删除", jobExecutingKey)
			delete(scheduler.jobPlanTable, jobExecutingKey)
		}

	case common.JOB_EVENT_KILL: // 杀掉job事件
		// 取消Command执行
		// log.Println(scheduler.jobExecutingTable)
		//jobExecutingKey = jobEvent.Job.Category + "-" + jobEvent.Job.Name
		jobExecutingKey = fmt.Sprintf("%s-%d", jobEvent.Job.Category, jobEvent.Job.ID)
		if jobExecuteInfo, isExist = scheduler.jobExecutingTable[jobExecutingKey]; isExist {
			// 是的在本work中执行中，那么可以杀掉它
			log.Println("需要kill job:", jobExecutingKey)
			// 执行计划任务执行信息中的取消函数
			// 修改执行信息的状态为kill
			jobExecuteInfo.Status = "kill"
			jobExecuteInfo.ExceteCancelFun()
		} else {
			// log.Println(scheduler.jobExecutingTable)
			log.Println("Job未在执行中，无需kill:", jobExecutingKey)
		}

	}
}

// 执行计划任务
func (scheduler *Scheduler) TryRunJob(jobPlan *datamodels.JobSchedulePlan) (err error) {
	var (
		jobExecuteInfo  *datamodels.JobExecuteInfo
		jobExecutingKey string
		isExecuting     bool
	)
	// 如果任务正在执行，跳过本次调度
	jobExecutingKey = fmt.Sprintf("%s-%d", jobPlan.Job.Category, jobPlan.Job.ID)
	if jobExecuteInfo, isExecuting = scheduler.jobExecutingTable[jobExecutingKey]; isExecuting {
		//log.Println("尚未退出，还在执行，跳过！", jobExecutingKey)
		return
	} else {
		// 构建执行状态信息
		jobExecuteInfo = common.BuildJobExecuteInfo(jobPlan)
		// 保存执行信息
		//jobExecutingKey = jobPlan.Job.Category + "-" + jobPlan.Job.Name
		scheduler.jobExecutingTable[jobExecutingKey] = jobExecuteInfo
		// 执行计划任务
		executor.ExecuteJob(jobExecuteInfo, scheduler.jobResultChan)
	}

	// 执行完毕后，从执行信息表中删除这条数据,这个在HandlerJobExecuteResult中处理
	// 即使未获取到锁，也需要从scheduler.jobExecutingTable 删除这条jobExecuteInfo
	return
}

// 回传任务执行结果
func (scheduler *Scheduler) PushJobExecuteResult(result *datamodels.JobExecuteResult) {
	scheduler.jobResultChan <- result
}

// 消费计划任务执行结果的循环
// 循环从jobExecuteResult中读取执行的结果
// 读取到结果后，交给HandlerJobExecuteResult处理
func (scheduler *Scheduler) comsumeJobExecuteResultsLoop() {
	var (
		result *datamodels.JobExecuteResult
	)
	for {
		select {
		case result = <-scheduler.jobResultChan:
			scheduler.HandlerJobExecuteResult(result)
		}
	}
}

// 处理计划任务的结果
func (scheduler *Scheduler) HandlerJobExecuteResult(result *datamodels.JobExecuteResult) {
	var (
		jobExecutingKey string
		jobExecuteLog   *datamodels.JobExecuteLog
	)
	// 删掉执行状态
	jobExecutingKey = fmt.Sprintf("%s-%d", result.ExecuteInfo.Job.Category, result.ExecuteInfo.Job.ID)
	//delete(scheduler.jobExecutingTable, result.ExecuteInfo.Job.Name)
	delete(scheduler.jobExecutingTable, jobExecutingKey)

	// 当前调度的任务，是否执行了
	// 没抢到执行锁，就不会执行，无需处理结果
	if result.IsExecuted {
		// 插入到Mongodb中，并更新执行的log_id
		if jobExecute, err := app.JobExecuteRepo.SaveExecuteLog(result); err != nil {
			log.Println("保存执行日志结果出错", err)
		} else {
			//log.Println(jobExecute)
			jobExecute = jobExecute
		}

		// 记录日志
		jobExecuteLog = &datamodels.JobExecuteLog{
			JobExecuteID: result.ExecuteID,
			Output:       string(result.Output),
		}

		// 判断是否有错误信息
		if result.Err != nil {
			jobExecuteLog.Err = result.Err.Error()
		}

		// 交给写日志的程序处理【异步去处理】[交给logHandler处理]
		//scheduler.logHandler.AddLog(jobExecuteLog)

		log.Printf("Job: %s执行完成：%s", jobExecutingKey, result.ExecuteInfo.Job.Command)
		// fmt.Println(string(result.Output))
		if result.Err != nil {
			log.Printf("%s执行出现了错误：%s\n", jobExecutingKey, result.Err.Error())
		}

	} else {
		// log.Printf("Job: %s 未执行：%s\n", result.ExecuteInfo.Job.Name, result.Err.Error())
	}
}

// 消费结果

// 初始化调度器
func NewScheduler() *Scheduler {
	var (
	//logHandler *MongoLogHandler
	//err error
	)
	// 实例化消息处理
	//if logHandler, err = NewMongoLogHandler(common.Config.Worker.Mongo); err != nil {
	//	log.Panic(err)
	//	return nil
	//} else {
	//
	//}
	scheduler := &Scheduler{
		jobEventChan:      make(chan *datamodels.JobEvent, 1000),
		jobPlanTable:      make(map[string]*datamodels.JobSchedulePlan),
		jobExecutingTable: make(map[string]*datamodels.JobExecuteInfo),
		jobResultChan:     make(chan *datamodels.JobExecuteResult, 500),
		isStoped:          false,
		//logHandler:        logHandler,
	}

	return scheduler
}
