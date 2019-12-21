package repositories

import (
	"fmt"
	"log"
	"testing"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
)

func TestWorkerRepository_Create(t *testing.T) {
	// 1. get db
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewWorkerRepository(etcd)

	// 3. 创建10个worker
	i := 0
	for i < 10 {
		i++
		name := fmt.Sprintf("Worker:%d", i)
		worker := &datamodels.Worker{
			Name: name,
			Host: "192.168.1.1",
			User: "root",
			Ip:   "192.168.1.1",
			Port: 900,
			Pid:  900 + i,
		}

		if worker, err := r.Create(worker); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(worker)
		}
	}
}

func TestWorkerRepository_List(t *testing.T) {
	// 1. get db
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewWorkerRepository(etcd)

	// 3. list worker
	if workers, err := r.List(); err != nil {
		t.Error(err.Error())
	} else {
		for i, worker := range workers {
			log.Println(i, worker)
		}
	}
}

func TestWorkerRepository_Delete(t *testing.T) {
	// 1. get db
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewWorkerRepository(etcd)

	// 3. Delete Worker
	i := 0
	for i < 10 {
		i++
		if i%2 == 0 {
			continue
		}

		// 执行删除
		name := fmt.Sprintf("Worker:%d", i)
		worker := &datamodels.Worker{
			Name: name,
		}

		if success, err := r.Delete(worker); err != nil {
			t.Error(err.Error())
		} else {
			if success {
				log.Println("删除成功：", name)

			} else {
				log.Println("删除失败！：", name)
			}
		}
	}
}
