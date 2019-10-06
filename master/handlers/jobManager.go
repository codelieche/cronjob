package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go.etcd.io/etcd/mvcc/mvccpb"

	"github.com/codelieche/cronjob/common"

	"go.etcd.io/etcd/clientv3"
)

// Job Manager
// 计划任务的管理器
type JobManager struct {
	client *clientv3.Client // etcd的客户端连接
	kv     clientv3.KV      // etcd的KV对
	lease  clientv3.Lease   // etcd的租约
}

// 保存Job到etcd中
// 返回上一次的Job和错误信息
func (jobManager *JobManager) SaveJob(job *common.Job) (prevJob *common.Job, err error) {
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
func (jobManager *JobManager) ListJobs() (jobList []*common.Job, err error) {
	// 定义变量
	var (
		jobsDirKey  string
		getResponse *clientv3.GetResponse
		kvPair      *mvccpb.KeyValue
		job         *common.Job
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
		job = &common.Job{}
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
func (jobManager *JobManager) GetJob(name string) (job *common.Job, err error) {
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
	err = fmt.Errorf("Job Not Fount!")
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

// 实例化Job Manager
func NewJobManager() (*JobManager, error) {
	var (
		config     clientv3.Config
		client     *clientv3.Client
		kv         clientv3.KV
		lease      clientv3.Lease
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

	//	实例化Job Manager
	jobManager = &JobManager{
		client: client,
		kv:     kv,
		lease:  lease,
	}

	// 返回
	return jobManager, nil
}
