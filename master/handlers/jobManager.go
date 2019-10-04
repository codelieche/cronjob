package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
