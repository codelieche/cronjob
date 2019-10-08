package worker

import (
	"log"

	"github.com/codelieche/cronjob/backend/common"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type WatchJobsHandler struct {
	KeyDir    string     // 监听的key目录
	Scheduler *Scheduler // 调度器
}

func (watch *WatchJobsHandler) HandlerGetResponse(response *clientv3.GetResponse) {
	var (
		job      *common.Job
		kvPair   *mvccpb.KeyValue
		err      error
		jobEvent *common.JobEvent
	)
	// for循环打印一下jobs
	for _, kvPair = range response.Kvs {
		if job, err = common.UnpackByteToJob(kvPair.Value); err != nil {
			log.Println(err.Error())
			continue
		} else {
			// 把这个job同步给scheduler
			//  log.Println(job)

			// 添加jobEvent
			jobEvent = &common.JobEvent{
				Event: common.JOB_EVENT_PUT,
				Job:   job,
			}

			// 加入到jobEventChan中
			watch.Scheduler.jobEventChan <- jobEvent

		}
	}
}

func (watch *WatchJobsHandler) HandlerWatchChan(watchChan clientv3.WatchChan) {
	var (
		job           *common.Job
		watchResponse clientv3.WatchResponse
		watchEvent    *clientv3.Event
		err           error
		jobEvent      *common.JobEvent
	)

	// 处理监听事件
	for watchResponse = range watchChan {
		for _, watchEvent = range watchResponse.Events {
			//log.Println("当前事件的Revision：", watchResponse.Header.Revision)
			switch watchEvent.Type {
			case mvccpb.PUT:
				//log.Printf(
				//	"监听到Put事件，Key: %s (IsCreate %v, IsModify %v)",
				//	string(watchEvent.Kv.Key), watchEvent.IsCreate(), watchEvent.IsModify(),
				//)
				// 反序列化，推送给调度协程
				if job, err = common.UnpackByteToJob(watchEvent.Kv.Value); err != nil {
					//log.Println(err.Error())
					log.Println(string(watchEvent.Kv.Value))
					continue
				} else {
					//log.Println("监听到新的job：", job)

					// 添加jobEvent
					jobEvent = &common.JobEvent{
						Event: common.JOB_EVENT_PUT,
						Job:   job,
					}

					// 加入到jobEventChan中
					watch.Scheduler.jobEventChan <- jobEvent
				}
			case mvccpb.DELETE:
				// 停止任务
				// 输出name
				log.Println("删除事件，Key：", string(watchEvent.Kv.Key))

				// 反序列化，推送给调度协程
				if job, err = common.UnpackByteToJob(watchEvent.PrevKv.Value); err != nil {
					//log.Println(err.Error())
					log.Println(string(watchEvent.Kv.Value))
					continue
				} else {
					log.Println("监听到删除job：", job)

					// 添加jobEvent
					jobEvent = &common.JobEvent{
						Event: common.JOB_EVENT_DELETE,
						Job:   job,
					}

					// 加入到jobEventChan中
					watch.Scheduler.jobEventChan <- jobEvent
				}
			}
		}
	}
}
