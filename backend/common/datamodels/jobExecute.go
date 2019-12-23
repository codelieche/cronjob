package datamodels

import (
	"context"
	"time"

	"github.com/gorhill/cronexpr"
)

// Job事件
type JobEvent struct {
	Event int      // 事件类型
	Job   *JobEtcd // Job
}

// Job调度计划
type JobSchedulePlan struct {
	Job        *JobEtcd             // 计划任务
	Expression *cronexpr.Expression // 解析好的cronexpr表达式
	NextTime   time.Time            // 下次执行时间
}

// Job执行信息
type JobExecuteInfo struct {
	Job             *JobEtcd           `json:"job"`            // 任务信息
	JobExecuteID    uint               `json:"job_execute_id"` // 任务执行ID
	PlanTime        time.Time          `json:"plan_time"`      // 计划调度的时间
	ExecuteTime     time.Time          `json:"execute_time"`   // 实际执行的时间
	ExecuteCtx      context.Context    `json:"-"`              // 执行job的上下文
	ExceteCancelFun context.CancelFunc `json:"-"`              // 执行执行job的取消函数
	Status          string             `json:"status"`         // 执行信息的状态：start、timeout、kill、success、error、done
}

// Job执行结果
type JobExecuteResult struct {
	ExecuteID   uint            // 任务执行的ID
	ExecuteInfo *JobExecuteInfo // 执行信息: 消费结果的时候，会根据这个来判断是否执行了
	IsExecuted  bool            // 是否有执行
	Output      []byte          // Job执行输出结果
	Err         error           // 脚本错误原因
	StartTime   time.Time       // 启动时间
	EndTime     time.Time       // 结束时间
	Status      string          // 执行状态：start、finish、cancel、success、error、timeout
}

// 任务调度前创建JobExecute
// 任务执行完后再写入JobExecuteLog信息
// Status：start、doing、cancel、done
type JobExecute struct {
	BaseFields
	Worker       string    `gorm:"size:100" json:"worker"`          // 执行者
	Category     string    `gorm:"size:100" json:"category"`        // 任务类型
	Name         string    `gorm:"size:100" json:"name"`            // 任务名字
	JobID        int       `gorm:"INDEX;NOT NULL" json:"job_id"`    // 计划任务ID
	Command      string    `gorm:"NOT NULL" json:"command"`         // 执行的命令
	Status       string    `gorm:"size:100;NOT NULL" json:"status"` // 任务执行状态
	PlanTime     time.Time `gorm:"NOT NULL" json:"plan_time"`       // 计划时间
	ScheduleTime time.Time `json:"schedule_time"`                   // 调度时间
	StartTime    time.Time `json:"start_time"`                      // 开始时间
	EndTime      time.Time `json:"end_time"`                        // 任务结束时间
	LogID        string    `json:"log_id"`                          // 执行结果保存的ObjectID
}

// 执行日志结果，写入到Mongodb中
type JobExecuteLog struct {
	JobExecuteID uint   `json:"job_execute_id" bson:"job_execute_id"` // 任务执行ID
	Output       string `json:"output" bson:"output"`                 // 执行任务输出结果
	Err          string `json:"err" bson:"err"`                       // 任务错误信息
	Success      bool   `json:"success" bson:"success"`               // 执行是否成功：当有错误日志的时候，就是未成功
}
