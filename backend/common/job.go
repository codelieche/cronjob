package common

import (
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
