package common

import (
	"context"
	"encoding/json"
	"errors"
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
		jobsDir  string
		jobKey   string
		jobValue []byte

		putResponse *clientv3.PutResponse
	)

	// 先处理下Job在etcd中的key
	if job.Name == "" {
		err = fmt.Errorf("Job的Name不能为空")
		return nil, err
	}

	// 如果job的分类为空，就设置其为default
	if job.Category == "" {
		job.Category = "default"
	}

	// jobKey = ETCD_JOBS_DIR + job.Name
	jobsDir = ETCD_JOBS_DIR
	if strings.HasSuffix(jobsDir, "/") {
		jobsDir = string(jobsDir[:len(jobsDir)-1])
	}

	// 组合jobKey
	jobKey = fmt.Sprintf("%s/%s/%s", jobsDir, job.Category, job.Name)

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
	jobsDirKey = ETCD_JOBS_DIR

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
func (jobManager *JobManager) GetJob(jobKey string) (job *Job, err error) {
	// 定义变量
	var (
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
		i           int
	)
	// 1. 对jobKey做校验
	jobKey = strings.TrimSpace(jobKey)
	if jobKey == "" {
		err = fmt.Errorf("传入的jobKey为空")
		return nil, err
	}
	if !strings.HasPrefix(jobKey, ETCD_JOBS_DIR) {
		err = errors.New("传入的key不正确")
		return false, err
	}

	// 2. 从etcd中获取对象
	jobKey = strings.Replace(jobKey, "//", "/", -1)
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
	err = fmt.Errorf("job not fount!")
	return nil, err
}

// Delete Job
func (jobManager *JobManager) DeleteJob(jobKey string) (success bool, err error) {
	// 定义变量
	var (
		deleteResponse *clientv3.DeleteResponse
	)
	// 1. 对key做判断
	jobKey = strings.TrimSpace(jobKey)
	if !strings.HasPrefix(jobKey, ETCD_JOBS_DIR) {
		err = errors.New("传入的key不正确")
		return false, err
	}

	if jobKey == "" {
		err = fmt.Errorf("jobKey不可为空")
		return false, err
	}

	// 2. 操作删除
	jobKey = strings.Replace(jobKey, "//", "/", -1)
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
	jobKillKey = ETCD_JOB_KILL_DIR + name

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
		putResponse = putResponse
		// log.Println(putResponse.Header.Revision)
	}

	return
}

// Watch keys
// 监听etcd key的变化: 比如监听jobs的变化，和监听kill的任务
// 传递的参数：要监听的key的前缀，和处理监听的接口
func (jobManager *JobManager) WatchKeys(keyDir string, watchHandler WatchHandler) (err error) {
	// 1. 定义变量
	var (
		getResponse *clientv3.GetResponse
		//kvPair             *mvccpb.KeyValue
		//job                *Job
		watchStartRevision int64
		watchChan          clientv3.WatchChan
		//watchResponse      clientv3.WatchResponse
		//watchEvent         *clientv3.Event
	)

	// 2. get：/crontab/jobs/目录下的所有任务，并且获知当前集群的revision
	//keyDir = "/crontab/jobs/"
	if getResponse, err = jobManager.kv.Get(
		context.TODO(), keyDir,
		clientv3.WithPrefix(),
	); err != nil {
		return
	}

	// 3. HandlerGetResponse(getResponse *clientv3.GetResponse)
	watchHandler.HandlerGetResponse(getResponse)

	// 4. watch新的变化
	func() { // 监听协程
		// 4-1: 从GET时刻后续版本开始监听
		watchStartRevision = getResponse.Header.Revision + 1
		log.Printf("开始watch事件:%s(Revision:%d)", keyDir, getResponse.Header.Revision)

		//	4-2：监听:/crontab/jobs/目录后续的变化
		watchChan = jobManager.watcher.Watch(
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

// 创建任务执行锁
func (jobManager *JobManager) CreateJobLock(name string) (jobLock *JobLock) {
	// 返回一把锁
	jobLock = NewJobLock(name, jobManager.kv, jobManager.lease)
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
