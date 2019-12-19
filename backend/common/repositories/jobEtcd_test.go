package repositories

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

func TestJobRepository_listJobsFromEtcd(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := jobRepository{
		db:   db,
		etcd: etcd,
	}

	// 3. 获取数据
	haveNext := true
	page := 1
	pageSize := 5
	for haveNext {
		if jobs, err := r.listJobsFromEtcd(page, pageSize); err != nil {
			t.Errorf(err.Error())
			haveNext = false
		} else {
			// 判断是否还有下一页
			//log.Println(len(jobs))
			if len(jobs) == pageSize && pageSize > 0 {
				log.Println("还有下一页")
				haveNext = true
				page += 1
			} else {
				log.Println("没有下一页了")
				haveNext = false
			}

			// 打印jobs
			for _, job := range jobs {
				log.Println(job.ID, job.Name, job.Time, job.Command)
			}
		}
	}
}
