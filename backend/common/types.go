package common

import (
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

// HTTP Response数据
type Response struct {
	Status  bool   `json:"status"`  // 状态
	Message string `json:"message"` // 消息
}

type WatchHandler interface {
	HandlerGetResponse(getResponse *clientv3.GetResponse) // 监听key之前先 Get一下所有的Key
	HandlerWatchChan(watchChan clientv3.WatchChan)        // 监听事件会有个watchResponse的Channel
}
