package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type WorkerRepository interface {
	// 创建Worker
	Create(worker *datamodels.Worker) (*datamodels.Worker, error)
	// 获取Worker
	Get(name string) (worker *datamodels.Worker, err error)
	// 删除Worker
	Delete(worker *datamodels.Worker) (success bool, err error)
	// 根据Worker的名字删除
	DeleteByName(name string) (success bool, err error)
	// 工作节点的列表
	List() (workersList []*datamodels.Worker, err error)
}

func NewWorkerRepository(etcd *datasources.Etcd) WorkerRepository {
	return &workerRepository{etcd: etcd}
}

type workerRepository struct {
	etcd *datasources.Etcd
}

// 创建worker
func (r *workerRepository) Create(worker *datamodels.Worker) (*datamodels.Worker, error) {
	// 定义变量
	var (
		workerEtcdKey  string
		workerEtcdData []byte
		putResponse    *clientv3.PutResponse
		err            error
	)
	// 直接把worker信息写入到etcd中
	worker.Name = strings.TrimSpace(worker.Name)
	if worker.Name == "" {
		err = errors.New("worker的名字不可为空")
		return nil, err
	}
	if worker.Name == "list" {
		err = errors.New("list是保留字，不可设置worker的name为list")
		return nil, err
	}

	// 开始写入到etcd中
	workerEtcdKey = common.ETCD_WORKER_DIR + worker.Name

	if workerEtcdData, err = json.Marshal(worker); err != nil {
		return nil, err
	}

	// 写入到etcd中
	if putResponse, err = r.etcd.KV.Put(
		context.Background(),
		workerEtcdKey,
		string(workerEtcdData),
		clientv3.WithPrevKV(),
	); err != nil {
		return nil, err
	} else {
		// 对结果进行判断

		putResponse = putResponse
	}
	return worker, nil
}

// 获取worker
func (r *workerRepository) Get(name string) (worker *datamodels.Worker, err error) {
	// 1. 定义变量
	var (
		etcdKey     string
		getResponse *clientv3.GetResponse
		keyValue    *mvccpb.KeyValue
	)

	// 2. 对name做校验
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("worker的Name不可为空")
		return nil, err
	}

	etcdKey = common.ETCD_WORKER_DIR + name

	// 3. 从etcd中获取对象
	if getResponse, err = r.etcd.KV.Get(
		context.Background(),
		etcdKey,
	); err != nil {
		return nil, err
	}
	// 4. 获取kv对象
	if len(getResponse.Kvs) == 1 {
		// 4-1: 获取etcd中的value
		keyValue = getResponse.Kvs[0]
		// 4-2: json反序列化
		worker = &datamodels.Worker{}
		if err = json.Unmarshal(keyValue.Value, worker); err != nil {
			log.Println("获取worker反序列化出错：", err)
			return nil, err
		} else {
			return worker, nil
		}
	} else {
		return nil, common.NotFountError
	}
}

// 删除worker
func (r *workerRepository) Delete(worker *datamodels.Worker) (success bool, err error) {
	return r.DeleteByName(worker.Name)
}

// 根据名字删除Worker
func (r *workerRepository) DeleteByName(name string) (success bool, err error) {
	// 定义变量
	var (
		etcdKey        string
		deleteResponse *clientv3.DeleteResponse
	)

	// 直接把worker信息写入到etcd中
	if name == "" {
		err = errors.New("worker的名字不可为空")
		return false, err
	}

	// 删除worker
	etcdKey = common.ETCD_WORKER_DIR + name

	if deleteResponse, err = r.etcd.KV.Delete(
		context.Background(),
		etcdKey,
		clientv3.WithPrevKV(),
	); err != nil {
		return false, err
	}
	// 校验被删除的KeyValue
	if len(deleteResponse.PrevKvs) < 1 {
		err = fmt.Errorf("%s不存在", etcdKey)
		return false, err
	} else {
		// 删除成功
		return true, nil
	}
}

// 获取worker的列表
func (r *workerRepository) List() (workersList []*datamodels.Worker, err error) {
	var (
		workerKeyDir string
		getResponse  *clientv3.GetResponse
		kvPair       *mvccpb.KeyValue
		worker       *datamodels.Worker
	)

	workerKeyDir = common.ETCD_WORKER_DIR

	// 从etcd中获取worker的列表
	if getResponse, err = r.etcd.KV.Get(context.TODO(),
		workerKeyDir, clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortAscend),
	); err != nil {
		return
	}

	// 对结果进行处理
	for _, kvPair = range getResponse.Kvs {
		worker = &datamodels.Worker{}
		// json处理
		if err = json.Unmarshal(kvPair.Value, worker); err != nil {
			log.Println(err.Error())
		} else {
			workersList = append(workersList, worker)
		}
	}
	//	处理完毕返回
	return
}
