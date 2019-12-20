package repositories

import (
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

func TestJobKillRepository_Save(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobKillRepository(db, etcd)

	// 3. 创建JobKill
	i := 0
	for i < 10 {
		i++
		job := &datamodels.JobKill{
			EtcdKey:    "",
			Category:   "default",
			JobID:      uint(i),
			Killed:     false,
			FinishedAt: nil,
			Result:     "",
		}
		if j, err := r.Save(job); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(j.ID, j.Category, j.JobID)
		}
	}
}

func TestJobKillRepository_List(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobKillRepository(db, etcd)

	// 3. 获取JobKill的列表
	haveNext := true
	offset := 0
	limit := 5

	for haveNext {
		if jobKills, err := r.List(offset, limit); err != nil {
			t.Error(err.Error())
			haveNext = false
		} else {
			// 判断是否还有下一页
			if len(jobKills) == limit && limit > 0 {
				haveNext = true
				offset += limit
			} else {
				haveNext = false
			}

			// 输出jobKill
			for _, jobKill := range jobKills {
				log.Println(jobKill.ID, jobKill.Category, jobKill.JobID)
			}
		}
	}
}

func TestJobKillRepository_Update(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobKillRepository(db, etcd)

	// 3. 创建JobKill
	i := 0
	for i < 3 {
		i++
		updatedFiled := make(map[string]interface{})
		updatedFiled["FinishedAt"] = time.Now()
		updatedFiled["Killed"] = true
		if jobKill, err := r.UpdateByID(int64(i), updatedFiled); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(jobKill.ID, jobKill.Category, jobKill.JobID)
		}
	}
}

func TestJobKillRepository_SetFinishedByID(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobKillRepository(db, etcd)

	// 3. 创建JobKill
	i := 6
	if jobKill, err := r.SetFinishedByID(int64(i)); err != nil {
		t.Error(err.Error())
	} else {
		log.Println("设置JobKill为Finished OK：", jobKill)
	}
}
