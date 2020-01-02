package datasources

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/interfaces"
	"github.com/coreos/etcd/clientv3"
)

// Watch keys
// 监听etcd key的变化: 比如监听jobs的变化，和监听kill的任务
// 传递的参数：要监听的key的前缀，和处理监听的接口
// keyDir: eg: /crontab/jobs/  监听前缀是jobs的key
// watchHandler：接口
//               1. HandlerGetResponse: 获取这些前缀的keyValue，然后做相应的处理
//               2. HandlerWatchChan：处理watchChan，比如修改了某个key，就有个WatchResponse推送到watchChan中
func (etcd *Etcd) WatchKeys(keyDir string, watchHandler interfaces.WatchHandler) (err error) {
	// 1. 定义变量
	var (
		getResponse *clientv3.GetResponse
		//kvPair             *mvccpb.KeyValue
		//job                *Job
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		//watchResponse      clientv3.WatchResponse
		//watchEvent         *clientv3.Event
		ctx context.Context
	)

	// 2. get：/crontab/jobs/目录下的所有任务，并且获知当前集群的revision
	//keyDir = "/crontab/jobs/"
	ctx, _ = context.WithTimeout(context.TODO(), time.Duration(common.GetConfig().Etcd.Timeout)*time.Millisecond)
	if getResponse, err = etcd.KV.Get(
		ctx, keyDir,
		clientv3.WithPrefix(),
	); err != nil {
		log.Println("执行watchKeys出错：", err)
		os.Exit(1)
		//return
	}

	// 3. HandlerGetResponse(getResponse *clientv3.GetResponse)
	watchHandler.HandlerGetResponse(getResponse)

	// 4. watch新的变化
	func() { // 监听协程
		// 4-1: 从GET时刻后续版本开始监听
		watchStartRevision = getResponse.Header.Revision + 1
		log.Printf("开始watch事件:%s(Revision:%d)", keyDir, getResponse.Header.Revision)

		//	4-2：监听:/crontab/jobs/目录后续的变化
		watchChan = etcd.Watcher.Watch(
			context.TODO(),
			keyDir,
			clientv3.WithPrefix(),                // 监听以jobKeyDir为前缀的key
			clientv3.WithRev(watchStartRevision), // 设置开始的版本号
			clientv3.WithPrevKV(),                // 如果不需知道上一次的值，可不添加这个option
		)

		// 4-3: 处理监听事件的Channel
		watchHandler.HandlerWatchChan(watchChan)

	}()
	return
}
