package repositories

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
)

func TestJobExecuteRepository_Create(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()
	mongoDB := datasources.GetMongoDB()

	// 2. init repository
	r := NewJobExecuteRepository(db, etcd, mongoDB)

	// 3. 创建JobExecute
	i := 0
	for i < 10 {
		i++
		now := time.Now()
		jobExecute := &datamodels.JobExecute{
			Worker:       "test worker",
			Category:     "default",
			Name:         "00001",
			JobID:        i,
			Command:      "echo `date`",
			Status:       "start",
			PlanTime:     now,
			ScheduleTime: now.Add(time.Second * 10),
			StartTime:    now.Add(time.Second * 11),
			EndTime:      now.Add(time.Minute),
			LogID:        "",
		}

		if jobExecute, err := r.Create(jobExecute); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(jobExecute.ID, jobExecute.Worker, jobExecute.Category, jobExecute.JobID, jobExecute.Status)
		}
	}
}

func TestJobExecuteRepository_List(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	mongoDB := datasources.GetMongoDB()

	// 2. init repository
	r := NewJobExecuteRepository(db, mongoDB)

	// 3. List JobExecute
	haveNext := true
	offset := 0
	limit := 5

	for haveNext {
		if jobExecutes, err := r.List(offset, limit); err != nil {
			haveNext = false
			t.Error(err.Error())
		} else {
			// 判断是否还有下一页
			if len(jobExecutes) == limit && limit > 0 {
				haveNext = true
				offset += limit
			} else {
				haveNext = false
			}

			// 打印JobExecute
			for _, jobExecute := range jobExecutes {
				log.Println(jobExecute)
			}
		}
	}
}

func TestJobExecuteRepository_UpdateByID(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()
	mongoDB := datasources.GetMongoDB()

	// 2. init repository
	r := NewJobExecuteRepository(db, etcd, mongoDB)

	// 3. 更新JobExecute

	var i int64 = 0
	for i < 10 {
		i++
		updateFields := make(map[string]interface{})
		updateFields["Status"] = "done"
		updateFields["LogID"] = fmt.Sprintf("mongoid-%d", i)
		if jobExecute, err := r.UpdateByID(i, updateFields); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(jobExecute)
		}
	}

}

func TestJobExecuteRepository_SaveExecuteLog(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()
	mongoDB := datasources.GetMongoDB()

	// 2. init repository
	r := NewJobExecuteRepository(db, etcd, mongoDB)

	// 3. 保存执行日志
	now := time.Now()

	i := 0

	for i < 10 {
		i++

		var output []byte
		output = []byte(fmt.Sprintf("这个是测试内容:%d", i))

		jobExecuteResult := &datamodels.JobExecuteResult{
			ExecuteID:  i,
			IsExecuted: true,
			Output:     output,
			Err:        nil,
			StartTime:  now,
			EndTime:    now.Add(time.Minute),
		}

		if jobExecute, err := r.SaveExecuteLog(jobExecuteResult); err != nil {
			t.Error(err.Error())
		} else {
			log.Println(jobExecute)
		}
	}

}

func TestJobExecuteRepository_GetExecuteLog(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()
	mongoDB := datasources.GetMongoDB()

	// 2. init repository
	r := NewJobExecuteRepository(db, etcd, mongoDB)

	// 3. get log
	var i int64 = 1
	if jobExecuteLog, err := r.GetExecuteLogByID(i); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(jobExecuteLog)
	}
}
