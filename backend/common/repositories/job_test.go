package repositories

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

func TestJobRepository_Save(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	rCategory := NewCategoryRepository(db, etcd)
	r := NewJobRepository(db, etcd)

	// 3. 插入10条Job
	// 3-1: 获取默认的分类
	var (
		category *datamodels.Category
		err      error
	)
	if category, err = rCategory.GetByName("default"); err != nil {
		//if category, err = rCategory.Get(2); err != nil {
		if err == common.NotFountError {
			// 创建默认分类
			category := &datamodels.Category{
				Name:        "default",
				IsActive:    true,
				CheckCmd:    "which bash",
				SetupCmd:    "echo `date`",
				TearDownCmd: "echo `date`",
			}
			// 插入Category
			if category, err = rCategory.Save(category); err != nil {
				t.Error(err.Error())
			} else {
				log.Println("插入分类：", category)
			}
		} else {
			t.Error(err.Error())
		}
	}

	// 3-2：插入分类
	i := 0
	for i < 10 {
		i += 1
		job := &datamodels.Job{
			Category:    category,
			CategoryID:  category.ID,
			Name:        fmt.Sprintf("Test Job %d", i),
			Time:        "*/5 * * * * * *",
			Command:     "echo `date`",
			Description: fmt.Sprintf("Test Job(%d) Description.", i),
			IsActive:    true,
			SaveOutput:  true,
		}

		// 3-3：保存到数据库
		if job, err := r.Save(job); err != nil {
			t.Error(err.Error())
		} else {
			log.Println("插入Job成功：", job)
		}
	}
}

// 获取Job的列表
func TestJobRepository_List(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobRepository(db, etcd)

	// 3. list jobs
	var (
		haveNext bool = true
		offset   int  = 0
		limit    int  = 5
	)
	for haveNext {
		if jobs, err := r.List(offset, limit); err != nil {
			haveNext = false
			t.Error(err.Error())
		} else {
			// 判断是否还有下一页
			if len(jobs) == limit && limit > 0 {
				haveNext = true
				offset += limit
			} else {
				haveNext = false
			}
			// 4. 打印job信息
			for _, job := range jobs {
				log.Println(job.ID, job.CategoryID, job.Name, job.Time, job.Command, job.Description, job.IsActive)
			}
		}
	}
}

func TestJobRepository_Update(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	rCategory := NewCategoryRepository(db, etcd)
	r := NewJobRepository(db, etcd)

	// 3. 获取Job
	var (
		job *datamodels.Job
		err error
		id  int64
	)
	id = 7
	if job, err = r.Get(id); err != nil {
		t.Error(err.Error())
		os.Exit(1)
	} else {
		// 获取到job
	}

	// 4. 更新job
	c, _ := rCategory.Get(11)
	fields := map[string]interface{}{
		"Description": "新的描述信息",
		"Command":     "echo `date`; sleep 10; echo `date`",
		"Category":    c,
		//"CategoryID":  1,
	}
	if job, err = r.Update(job, fields); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(job)
	}
}

func TestJobRepository_Delete(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewJobRepository(db, etcd)

	// 3. Delete
	// 3-1: 获取job
	var (
		job *datamodels.Job
		err error
	)
	if job, err = r.Get(7); err != nil {
		t.Error(err.Error())
		os.Exit(1)
	} else {
		// 3-2: Delete Job
		if err = r.Delete(job); err != nil {
			t.Error(err.Error())
		} else {
			log.Println("删除Job成功：", job)
		}
	}
}
