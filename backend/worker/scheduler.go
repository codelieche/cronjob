package worker

import (
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
)

// 任务调度器
type Scheduler struct {
	jobEventChan chan *common.JobEvent              // etcd任务时间队列
	jobPlanTable map[string]*common.JobSchedulePlan // 任务调度计划表
}

// 计算任务调度状态
func (scheduler *Scheduler) TrySchedule() (scheduleAfter time.Duration) {
	var (
		jobPlan  *common.JobSchedulePlan
		now      time.Time
		nearTime *time.Time
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
			log.Println("执行计划任务：", jobPlan.Job.Name)
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

	}
}

// 初始化调度器
func NewScheduler() *Scheduler {
	scheduler := &Scheduler{
		jobEventChan: make(chan *common.JobEvent, 1000),
		jobPlanTable: make(map[string]*common.JobSchedulePlan),
	}
	return scheduler
}
