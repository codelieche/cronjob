package common

import (
	"time"

	"github.com/gorhill/cronexpr"
	"go.etcd.io/etcd/clientv3"
)

// 定时任务
// 比如：每三十分钟执行一次的任务
// */30 * * * * echo `date` >> /var/log/test.log
type Job struct {
	Name        string `json:"name"`                  // 任务的名称
	Time        string `json:"time"`                  // 计划任务的时间
	Command     string `json:"command"`               // 任务的命令
	Description string `json:"description,omitempty"` // Job描述
}

// Job Manager
// 计划任务的管理器
type JobManager struct {
	client  *clientv3.Client // etcd的客户端连接
	kv      clientv3.KV      // etcd的KV对
	lease   clientv3.Lease   // etcd的租约
	watcher clientv3.Watcher // etcd watch
}

// Job事件
type JobEvent struct {
	Event int  // 事件类型
	Job   *Job // Job
}

// Job调度计划
type JobSchedulePlan struct {
	Job        *Job                 // 计划任务
	Expression *cronexpr.Expression // 解析好的cronexpr表达式
	NextTime   time.Time            // 下次执行时间
}

// Job执行信息
type JobExecuteInfo struct {
	Job         *Job      // 任务信息
	PlanTime    time.Time // 计划调度的时间
	ExecuteTime time.Time // 实际执行的时间
}

// Job执行结果
type JobExecuteResult struct {
	ExecuteInfo *JobExecuteInfo // 执行信息: 消费结果的时候，会根据这个来判断是否执行了
	IsExecute   bool            // 是否有执行
	Output      []byte          // Job执行输出结果
	Err         error           // 脚本错误原因
	StartTime   time.Time       // 启动时间
	EndTime     time.Time       // 结束时间
}

// HTTP Response数据
type Response struct {
	Status  bool   `json:"status"`  // 状态
	Message string `json:"message"` // 消息
}

type WatchHandler interface {
	HandlerGetResponse(getResponse *clientv3.GetResponse) // 监听key之前先 Get一下所有的Key
	HandlerWatchChan(watchChan clientv3.WatchChan)        // 监听事件会有个watchResponse的Channel
}
