package common

import (
	"log"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// 监听kill处理器Demo
type WatchKillHandlerDemo struct {
}

func (demo *WatchKillHandlerDemo) HandlerGetResponse(response *clientv3.GetResponse) {
	var (
		kvPair *mvccpb.KeyValue
	)
	// for循环打印一下jobs
	for _, kvPair = range response.Kvs {
		log.Println(string(kvPair.Key), string(kvPair.Value))
	}
}

func (demo *WatchKillHandlerDemo) HandlerWatchChan(watchChan clientv3.WatchChan) {
	var (
		watchResponse clientv3.WatchResponse
		watchEvent    *clientv3.Event
	)

	// 处理监听事件
	for watchResponse = range watchChan {
		for _, watchEvent = range watchResponse.Events {
			//log.Println("当前事件的Revision：", watchResponse.Header.Revision)
			switch watchEvent.Type {
			case mvccpb.PUT:
				log.Printf(
					"监听到kill Put事件，Key: %s (IsCreate %v, IsModify %v), Value: %s",
					string(watchEvent.Kv.Key), watchEvent.IsCreate(), watchEvent.IsModify(),
					string(watchEvent.Kv.Value),
				)

			case mvccpb.DELETE:
				// key删除
				log.Printf("kill delete，Key：%s, Value: %s\n",
					string(watchEvent.Kv.Key), string(watchEvent.Kv.Value),
				)

			}
		}
	}
}

// 从etcd jobs key中提取JobName
// 比如：/crontab/jobs/test 得到的jobName是test
func ExtractJobName(jobKey string) string {
	return strings.TrimPrefix(jobKey, ETCD_JOBS_DIR)
}

// 从etcd kill key中提取jobName
// 比如：/crontab/kill/test 得到的jobName是test
func ExtractKillJobName(killKey string) string {
	return strings.TrimPrefix(killKey, ETCD_JOB_KILL_DIR)
}
