package common

import (
	"log"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type WatchJobsHandlerDemo struct {
	KeyDir string // 监听的key目录
}

func (demo *WatchJobsHandlerDemo) HandlerGetResponse(response *clientv3.GetResponse) {
	var (
		job    *Job
		kvPair *mvccpb.KeyValue
		err    error
	)
	// for循环打印一下jobs
	for _, kvPair = range response.Kvs {
		if job, err = UnpackByteToJob(kvPair.Value); err != nil {
			log.Println(err.Error())
			continue
		} else {
			// 把这个job同步给scheduler
			log.Println(job)
		}
	}
}

func (demo *WatchJobsHandlerDemo) HandlerWatchChan(watchChan clientv3.WatchChan) {
	var (
		job           *Job
		watchResponse clientv3.WatchResponse
		watchEvent    *clientv3.Event
		err           error
	)

	// 处理监听事件
	for watchResponse = range watchChan {
		for _, watchEvent = range watchResponse.Events {
			log.Println("当前事件的Revision：", watchResponse.Header.Revision)
			switch watchEvent.Type {
			case mvccpb.PUT:
				log.Printf(
					"监听到Put事件，Key: %s (IsCreate %v, IsModify %v)",
					string(watchEvent.Kv.Key), watchEvent.IsCreate(), watchEvent.IsModify(),
				)
				// 反序列化，推送给调度协程
				if job, err = UnpackByteToJob(watchEvent.Kv.Value); err != nil {
					//log.Println(err.Error())
					log.Println(string(watchEvent.Kv.Value))
					continue
				} else {
					log.Println("监听到新的job：", job)
				}
			case mvccpb.DELETE:
				// 停止任务
				// 输出name
				log.Println("删除事件，Key：", string(watchEvent.Kv.Key))

				// 反序列化，推送给调度协程
				if job, err = UnpackByteToJob(watchEvent.PrevKv.Value); err != nil {
					//log.Println(err.Error())
					log.Println(string(watchEvent.Kv.Value))
					continue
				} else {
					log.Println("监听到删除job：", job)
				}
			}
		}
	}
}
