package common

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"go.etcd.io/etcd/mvcc/mvccpb"

	"go.etcd.io/etcd/clientv3"
)

// Worker管理器
type WorkerManager struct {
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease
}

// 获取worker的列表
func (workerManager *WorkerManager) ListWorekr() (workerList []*WorkerInfo, err error) {
	var (
		workerKeyDir string
		getResponse  *clientv3.GetResponse
		kvPair       *mvccpb.KeyValue
		workerInfo   *WorkerInfo
	)

	workerKeyDir = ETCD_WORKER_DIR

	// 从etcd中获取worker的列表
	if getResponse, err = workerManager.kv.Get(context.TODO(),
		workerKeyDir, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortDescend),
	); err != nil {
		return
	}

	// 对结果进行处理
	for _, kvPair = range getResponse.Kvs {
		workerInfo = &WorkerInfo{}
		// json处理
		if err = json.Unmarshal(kvPair.Value, workerInfo); err != nil {
			log.Println(err.Error())
		} else {
			workerList = append(workerList, workerInfo)
		}
	}
	//	处理完毕返回
	return
}

// 创建个新的workerManager
func NewWorkerManager(etcdConfig *EtcdConfig) (workerManager *WorkerManager, err error) {

	var (
		config clientv3.Config
		client *clientv3.Client
		kv     clientv3.KV
		lease  clientv3.Lease
	)

	config = clientv3.Config{
		//Endpoints:   []string{"127.0.0.1:2379"},
		Endpoints:   etcdConfig.Endpoints,
		DialTimeout: 10 * time.Second,
	}

	if client, err = clientv3.New(config); err != nil {
		return
	}

	kv = clientv3.NewKV(client)
	lease = clientv3.NewLease(client)

	workerManager = &WorkerManager{
		client: client,
		kv:     kv,
		lease:  lease,
	}

	return
}
