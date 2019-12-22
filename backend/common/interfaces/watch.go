package interfaces

import "github.com/coreos/etcd/clientv3"

type WatchHandler interface {
	HandlerGetResponse(getResponse *clientv3.GetResponse) // 监听key之前先 Get一下所有的Key
	HandlerWatchChan(watchChan clientv3.WatchChan)        // 监听事件会有个watchResponse的Channel
}
