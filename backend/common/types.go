package common

import (
	"github.com/coreos/etcd/clientv3"
)

// Job Manager
// 计划任务的管理器
type EtcdManager struct {
	client  *clientv3.Client // etcd的客户端连接
	kv      clientv3.KV      // etcd的KV对
	lease   clientv3.Lease   // etcd的租约
	watcher clientv3.Watcher // etcd watch
}

// Kill计划任务的info
type KillJob struct {
	Category string `json:"category"` // 要杀掉的job的分类
	Name     string `json:"name"`     // 要杀掉的job的名称
}

type WatchHandler interface {
	HandlerGetResponse(getResponse *clientv3.GetResponse) // 监听key之前先 Get一下所有的Key
	HandlerWatchChan(watchChan clientv3.WatchChan)        // 监听事件会有个watchResponse的Channel
}
