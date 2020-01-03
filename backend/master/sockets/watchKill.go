package sockets

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type WatchKillHandler struct {
	KeyDir string // 监听的key目录
	app    *App   // 调度器
}

func (watch *WatchKillHandler) HandlerGetResponse(response *clientv3.GetResponse) {
	var (
		kvPair *mvccpb.KeyValue
	)
	// for循环打印一下jobs
	for _, kvPair = range response.Kvs {
		// 打印一下
		log.Println(string(kvPair.Key), string(kvPair.Value))
	}
}

func (watch *WatchKillHandler) HandlerWatchChan(watchChan clientv3.WatchChan) {
	var (
		watchResponse clientv3.WatchResponse
		watchEvent    *clientv3.Event
		jobName       string
		killJob       *datamodels.JobKill
		job           *datamodels.JobEtcd
		jobEvent      *datamodels.JobEvent
		err           error
	)

	// 处理监听事件
	for watchResponse = range watchChan {
		for _, watchEvent = range watchResponse.Events {
			//log.Println("当前事件的Revision：", watchResponse.Header.Revision)
			switch watchEvent.Type {
			case mvccpb.PUT:
				// 杀死Job任务事件
				killJob = &datamodels.JobKill{}
				if err = json.Unmarshal(watchEvent.Kv.Value, killJob); err != nil {
					log.Println(string(watchEvent.Kv.Value))
					log.Println(err)
					continue
				}

				// 从key中提取出jobName
				//待删 jobName = common.ExtractKillJobName(string(watchEvent.Kv.Key))
				//log.Println(jobName, "===>", string(watchEvent.Kv.Key))
				jobName = strconv.Itoa(int(killJob.JobID))
				job = &datamodels.JobEtcd{
					ID:       killJob.JobID, // Job的ID
					Category: killJob.Category,
					Name:     jobName, // Name可以去掉
				}
				// 构造JobEvnet
				jobEvent = &datamodels.JobEvent{
					Event: common.JOB_EVENT_KILL,
					Job:   job,
				}

				// 发送jobEvent信息给clients
				watch.app.pushMessageEventToAllClients("jobEvent", jobEvent)

			case mvccpb.DELETE:
				// key删除: 我们可以不关心
				//log.Printf("kill delete，Key：%s, Value: %s\n",
				//	string(watchEvent.Kv.Key), string(watchEvent.Kv.Value),
				//)
			}
		}
	}
}
