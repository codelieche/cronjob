package worker

import (
	"log"

	"github.com/codelieche/cronjob/backend/common"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type WatchKillHandler struct {
	KeyDir    string     // 监听的key目录
	Scheduler *Scheduler // 调度器
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
		job           *common.Job
		jobEvent      *common.JobEvent
	)

	// 处理监听事件
	for watchResponse = range watchChan {
		for _, watchEvent = range watchResponse.Events {
			//log.Println("当前事件的Revision：", watchResponse.Header.Revision)
			switch watchEvent.Type {
			case mvccpb.PUT:
				// 杀死Job任务事件

				// 从key中提取出jobName
				jobName = common.ExtractKillJobName(string(watchEvent.Kv.Key))
				//log.Println(jobName, "===>", string(watchEvent.Kv.Key))
				job = &common.Job{
					Name: jobName,
				}
				// 构造JobEvnet
				jobEvent = &common.JobEvent{
					Event: common.JOB_EVENT_KILL,
					Job:   job,
				}

				// 加入到jobEventChan中，
				// 在scheduler.ScheduleLoop()中会根据select响应events
				// 在scheduler.handleJobEvent中会根据事件做相应的操作
				// watch.Scheduler.jobEventChan <- jobEvent
				watch.Scheduler.PushJobEvent(jobEvent)

				//log.Printf(
				//	"监听到kill Put事件，Key: %s (IsCreate %v, IsModify %v), Value: %s",
				//	string(watchEvent.Kv.Key), watchEvent.IsCreate(), watchEvent.IsModify(),
				//	string(watchEvent.Kv.Value),
				//)

			case mvccpb.DELETE:
				// key删除: 我们可以不关心
				//log.Printf("kill delete，Key：%s, Value: %s\n",
				//	string(watchEvent.Kv.Key), string(watchEvent.Kv.Value),
				//)
			}
		}
	}
}
