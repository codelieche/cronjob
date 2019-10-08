package common

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorhill/cronexpr"
)

// 反序列化Job
func UnpackByteToJob(value []byte) (job *Job, err error) {

	// 直接用json反序列化
	if err = json.Unmarshal(value, &job); err != nil {
		return
	} else {
		return job, nil
	}
}

// 构建job执行计划
func BuildJobSchedulePlan(job *Job) (jobSchedulePlan *JobSchedulePlan, err error) {
	var (
		expression *cronexpr.Expression
		now        time.Time
	)
	// 解析job的cron表达式
	if expression, err = cronexpr.Parse(job.Time); err != nil {
		return
	}

	// 生成job调度计划对象
	now = time.Now()
	jobSchedulePlan = &JobSchedulePlan{
		Job:        job,
		Expression: expression,
		NextTime:   expression.Next(now),
	}
	return jobSchedulePlan, nil
}

// 构造执行状态信息
func BuildJobExecuteInfo(jobPlan *JobSchedulePlan) (jobExecuteInfo *JobExecuteInfo) {
	jobExecuteInfo = &JobExecuteInfo{
		Job:         jobPlan.Job,
		PlanTime:    jobPlan.NextTime,
		ExecuteTime: time.Now(),
	}
	// 为本次的执行创建一个执行上下文：主要用于取消job的执行
	jobExecuteInfo.ExecuteCtx, jobExecuteInfo.ExceteCancelFun = context.WithCancel(context.TODO())

	return
}
