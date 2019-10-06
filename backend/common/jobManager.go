package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go.etcd.io/etcd/mvcc/mvccpb"

	"go.etcd.io/etcd/clientv3"
)

// 保存Job到etcd中
// 返回上一次的Job和错误信息
func (jobManager *JobManager) SaveJob(job *Job) (prevJob *Job, err error) {
	// 把任务保存到/crontab/jobs/:name中
	var (
		jobKey   string
		jobValue []byte

		putResponse *clientv3.PutResponse
	)

	// 先处理下Job在etcd中的key
	if job.Name == "" {
		err = fmt.Errorf("Job的Name不能为空")
		return nil, err
	}

	jobKey = fmt.Sprintf("/crontab/jobs/%s", job.Name)

	// 任务信息json：对job序列化一下
	if jobValue, err = json.Marshal(job); err != nil {
		return nil, err
	}

	//	保存到etcd中
	if putResponse, err = jobManager.kv.Put(
		context.TODO(),        // 上下文
		jobKey,                // Key
		string(jobValue),      // 值
		clientv3.WithPrevKV(), // 返回上一个版本的值
	); err != nil {
		return nil, err
	}

	// 如果是更新，那么返回上一个版本的job
	if putResponse.PrevKv != nil {
		//	对旧值反序列化下
		if err = json.Unmarshal(putResponse.PrevKv.Value, &prevJob); err != nil {
			log.Println(err.Error())
			// 这里虽然反序列化出错了，但是不影响保存的操作，这里我们可以把err设置为空
			return nil, nil
		} else {
			// 返回上一个的旧值
			return prevJob, err
		}
	} else {
		// 没有上一个的job值，直接返回
		return nil, nil
	}
}

// List Jobs
func (jobManager *JobManager) ListJobs() (jobList []*Job, err error) {
	// 定义变量
	var (
		jobsDirKey  string
		getResponse *clientv3.GetResponse
		kvPair      *mvccpb.KeyValue
		job         *Job
	)
	jobsDirKey = "/crontab/jobs/"

	// 获取job对象
	//endKey := "/crontab/jobs/test2"
	//jobsDirKey = endKey

	// clientv3.WithFromKey() 会从传入的key开始获取，不可与WithPrefix同时使用
	if getResponse, err = jobManager.kv.Get(
		context.TODO(),
		jobsDirKey,
		clientv3.WithPrefix(),
		//clientv3.WithFromKey(),
		//clientv3.WithLimit(10),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	); err != nil {
		// 出错
		return
	}

	// 变量kv
	for _, kvPair = range getResponse.Kvs {
		//	对值序列化
		job = &Job{}
		if err = json.Unmarshal(kvPair.Value, job); err != nil {
			continue
		} else {
			jobList = append(jobList, job)
		}
	}

	//	返回结果
	return jobList, nil

}

// 获取Job的Detail
func (jobManager *JobManager) GetJob(name string) (job *Job, err error) {
	// 定义变量
	var (
		jobKey      string
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
	)
	// 1. 对name做校验
	name = strings.TrimSpace(name)
	if name == "" {
		err = fmt.Errorf("传入的name为空")
		return nil, err
	}

	// 2. 从etcd中获取对象
	jobKey = fmt.Sprintf("/crontab/jobs/%s", name)
	if getResponse, err = jobManager.kv.Get(context.TODO(), jobKey); err != nil {
		return nil, err
	}

	// 3. 获取kv对象
	//log.Println(getResponse.Header)
	//log.Println(getResponse.Kvs)
	if len(getResponse.Kvs) == 1 {
		for i = range getResponse.Kvs {
			keyValue = getResponse.Kvs[i]
			//log.Println(keyValue.Value)
			//	4. json反序列化
			if err = json.Unmarshal(keyValue.Value, &job); err != nil {
				return nil, err
			} else {
				return job, nil
			}
		}
		goto NotFound
	} else {
		goto NotFound
	}

NotFound:
	err = fmt.Errorf("job Not Fount!")
	return nil, err
}

// Delete Job
func (jobManager *JobManager) DeleteJob(name string) (success bool, err error) {
	// 定义变量
	var (
		jobKey         string
		deleteResponse *clientv3.DeleteResponse
	)
	// 1. 对name做判断
	name = strings.TrimSpace(name)

	if name == "" {
		err = fmt.Errorf("name不可为空")
		return false, err
	}

	// 2. 操作删除
	jobKey = fmt.Sprintf("/crontab/jobs/%s", name)
	if deleteResponse, err = jobManager.kv.Delete(
		context.TODO(),
		jobKey,
		clientv3.WithPrevKV(),
	); err != nil {
		return false, err
	}

	// 3. 返回被删除的keyValue
	if len(deleteResponse.PrevKvs) < 1 {
		err = fmt.Errorf("%s不存在", jobKey)
		return false, err
	} else {
		// 删除成功
		return true, nil
	}
}

// 计划任务kill
// 杀掉计划任务运行的进程
func (jobManager *JobManager) KillJob(name string) (err error) {
	// 添加要杀掉的Job信息
	// 通过在：/crontab/kill/:name添加一条数据
	// Worker节点，会监听到这个条目的PUT操作，然后做相应的操作

	// 1. 定义变量
	var (
		jobKillKey         string
		leaseGrantResponse *clientv3.LeaseGrantResponse
		leaseID            clientv3.LeaseID
		putResponse        *clientv3.PutResponse
	)

	// 校验key
	name = strings.TrimSpace(name)
	if name == "" {
		err = fmt.Errorf("job的name不可为空")
		return
	}
	jobKillKey = fmt.Sprintf("/crontab/kill/%s", name)

	// 2. 通知worker杀死对应的任务
	// 2-1: 创建个租约
	if leaseGrantResponse, err = jobManager.lease.Grant(context.TODO(), 5); err != nil {
		// 创建租约失败
		return
	}
	// 2-2： 得到租约ID
	leaseID = leaseGrantResponse.ID

	// 2-3: 添加kill记录
	if putResponse, err = jobManager.kv.Put(
		context.TODO(),
		jobKillKey, name,
		clientv3.WithLease(leaseID),
	); err != nil {
		return
	} else {
		// put成功
		//putResponse = putResponse
		log.Println(putResponse.Header.Revision)
	}

	return
}

// Watch Jobs
// 监听Job的变化
func (jobManager *JobManager) WatchJobs() (err error) {
	// 1. 定义变量
	var (
		jobKeyDir          string
		getResponse        *clientv3.GetResponse
		kvPair             *mvccpb.KeyValue
		job                *Job
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		watchResponse      clientv3.WatchResponse
		watchEvent         *clientv3.Event
	)

	// 2. get：/crontab/jobs/目录下的所有任务，并且获知当前集群的revision
	jobKeyDir = "/crontab/jobs/"
	if getResponse, err = jobManager.kv.Get(
		context.TODO(), jobKeyDir,
		clientv3.WithPrefix(),
	); err != nil {
		return
	}

	// 3. for循环打印一下jobs
	for _, kvPair = range getResponse.Kvs {
		if job, err = UnpackByteToJob(kvPair.Value); err != nil {
			log.Println(err.Error())
			continue
		} else {
			// 把这个job同步给scheduler
			log.Println(job)
		}
	}

	// 4. watch新的变化
	func() { // 监听协程
		// 4-1: 从GET时刻后续版本开始监听
		watchStartRevision = getResponse.Header.Revision + 1
		log.Println("开始watch事件:", watchStartRevision)

		//	4-2：监听:/crontab/jobs/目录后续的变化
		watchChan = jobManager.watcher.Watch(
			context.TODO(),
			jobKeyDir,
			clientv3.WithPrefix(),                // 监听以jobKeyDir为前缀的key
			clientv3.WithRev(watchStartRevision), // 设置开始的版本号
			clientv3.WithPrevKV(),                // 如果不需知道上一次的值，可不添加这个option
		)

		// 4-3: 处理监听事件
		for watchResponse = range watchChan {
			for _, watchEvent = range watchResponse.Events {
				log.Println("当前事件的Revision：", watchResponse.Header.Revision)
				switch watchEvent.Type {
				case mvccpb.PUT:
					log.Printf("监听到Put事件: IsCreate %v, IsModify %v", watchEvent.IsCreate(), watchEvent.IsModify())
					// 反序列化，推送给调度协程
					if job, err = UnpackByteToJob(watchEvent.Kv.Value); err != nil {
						log.Println(err.Error())
						continue
					} else {
						log.Println("监听到新的job：", job)
					}
				case mvccpb.DELETE:
					log.Println("删除事件")
					// 停止任务
					// 输出name
					log.Println("删除Key：", string(watchEvent.Kv.Key))

					// 反序列化，推送给调度协程
					if job, err = UnpackByteToJob(watchEvent.PrevKv.Value); err != nil {
						log.Println(err.Error())
						continue
					} else {
						log.Println("监听到删除job：", job)
					}
				}
			}
		}

	}()
	return
}

// 实例化Job Manager
func NewJobManager() (*JobManager, error) {
	var (
		config     clientv3.Config
		client     *clientv3.Client
		kv         clientv3.KV
		lease      clientv3.Lease
		watcher    clientv3.Watcher
		err        error
		jobManager *JobManager
	)

	//	初始化etcd配置
	config = clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"}, // 集群地址
		DialTimeout: 5000 * time.Microsecond,    // 连接超时

	}

	// 建立连接
	if client, err = clientv3.New(config); err != nil {
		return nil, err
	} else {
		// 连接成功
	}

	// 得到KV的Lease的API子集
	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)
	watcher = clientv3.NewWatcher(client)

	//	实例化Job Manager
	jobManager = &JobManager{
		client:  client,
		kv:      kv,
		lease:   lease,
		watcher: watcher,
	}

	// 返回
	return jobManager, nil
}
