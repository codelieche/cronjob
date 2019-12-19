package repositories

import (
	"errors"
	"log"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/jinzhu/gorm"
)

type JobRepository interface {
	// 保存Job
	Save(job *datamodels.Job) (*datamodels.Job, error)
	// 获取Job的列表
	List(offset int, limit int) (jobs []*datamodels.Job, err error)
	// 获取Job的信息
	Get(id int64) (job *datamodels.Job, err error)
	GetWithCategory(id int64) (job *datamodels.Job, err error)
	// 删除Job
	Delete(job *datamodels.Job) (err error)
	// 修改Job
	Update(job *datamodels.Job, fields map[string]interface{}) (*datamodels.Job, error)
	UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Job, error)
}

func NewJobRespository(db *gorm.DB, etcd *datasources.Etcd) JobRepository {
	return &jobRepository{
		db:   db,
		etcd: etcd,
		infoFields: []string{
			"id", "created_at", "updated_at", "deleted_at",
			"name", "category_id", "time", "command", "description", "is_active", "save_output",
		},
	}
}

type jobRepository struct {
	db         *gorm.DB
	etcd       *datasources.Etcd
	infoFields []string
}

// 保存Job
func (r *jobRepository) Save(job *datamodels.Job) (*datamodels.Job, error) {
	if job.ID > 0 {
		// 是更新操作
		if err := r.db.Model(&datamodels.Job{}).Update(job).Error; err != nil {
			return nil, err
		} else {
			// 需要更新一下etcd中的数据
			if prevJob, err := r.saveJobToEtcd(job, false); err != nil {
				// 保存去etcd出错
				// 当不存在的时候，就需要重新创建一下
				if err == common.NotFountError {
					// 不存在etcd中，我们需要创建一下
					if _, err = r.saveJobToEtcd(job, true); err != nil {
						log.Println("创建Job成功了，但是保存到etcd的时候，出错了", err.Error())
					}
				} else {
					log.Println("保存到mysql成功了，但是保存到etcd的时候，出错了", err.Error())
				}

			} else {
				log.Println(prevJob)
			}
			return job, nil
		}
	} else {
		// 是创建操作
		if err := r.db.Create(job).Error; err != nil {
			return nil, err
		} else {
			// 需要插入到etcd中
			// 需要更新一下etcd中的数据
			if prevJob, err := r.saveJobToEtcd(job, true); err != nil {
				// 保存去etcd出错
				log.Println("保存到mysql成功了，但是保存到etcd的时候，出错了", err.Error())

			} else {
				log.Println(prevJob)
			}

			return job, nil
		}
	}
}

func (r *jobRepository) List(offset int, limit int) (jobs []*datamodels.Job, err error) {
	query := r.db.Model(&datamodels.Job{}).Select(r.infoFields).Offset(offset).Limit(limit).Find(&jobs)
	if query.Error != nil {
		return nil, query.Error
	} else {
		return jobs, nil
	}
}

func (r *jobRepository) Get(id int64) (job *datamodels.Job, err error) {
	job = &datamodels.Job{}
	r.db.Select(r.infoFields).First(job, "id = ?", id)
	if job.ID > 0 {
		return job, nil
	} else {
		return nil, common.NotFountError
	}
}

func (r *jobRepository) GetWithCategory(id int64) (job *datamodels.Job, err error) {
	job = &datamodels.Job{}
	r.db.Select(r.infoFields).Preload("Category", func(d *gorm.DB) *gorm.DB {
		return d.Select("id, name, is_active")
	}).First(job, "id = ?", id)
	if job.ID > 0 {
		return job, nil
	} else {
		return nil, common.NotFountError
	}
}

func (r *jobRepository) Delete(job *datamodels.Job) (err error) {
	if job.IsActive {
		job.IsActive = false
		if job, err = r.Update(job, map[string]interface{}{"IsActive": false}); err != nil {
			return err
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (r *jobRepository) Update(job *datamodels.Job, fields map[string]interface{}) (*datamodels.Job, error) {
	// 判断ID：
	// 如果传入的是0，那么会更新全部
	// 如果fields中传入了ID，那么会更新ID是它的对象
	// 推荐加一个limit(1), 确保只更新一条数据
	if job.ID <= 0 {
		err := errors.New("传入ID为0，会更新全部数据")
		return nil, err
	}

	// 丢弃ID/Id/iD
	idKeys := []string{"ID", "id", "Id", "iD"}
	for _, k := range idKeys {
		if _, exist := fields[k]; exist {
			delete(fields, k)
		}
	}

	// 更新操作
	if err := r.db.Model(job).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		// 需要更新一下etcd中的数据
		if prevJob, err := r.saveJobToEtcd(job, false); err != nil {
			// 保存去etcd出错
			// 当不存在的时候，就需要重新创建一下
			if err == common.NotFountError {
				// 不存在etcd中，我们需要创建一下
				if _, err = r.saveJobToEtcd(job, true); err != nil {
					log.Println("创建Job成功了，但是保存到etcd的时候，出错了", err.Error())
				}
			} else {
				log.Println("保存到mysql成功了，但是保存到etcd的时候，出错了", err.Error())
			}

		} else {
			log.Println(prevJob)
		}

		return job, nil
	}
}

func (r *jobRepository) UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Job, error) {
	// 判断ID
	if id <= 0 {
		err := errors.New("传入的ID为0，会更新全部数据")
		return nil, err
	}

	// 更新操作
	if err := r.db.Model(&datamodels.Job{}).Where("id = ?", id).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		// 返回获取到的对象
		return r.Get(id)
	}
}

func (r *jobRepository) getOrCreateDefaultCategory() (category *datamodels.Category, err error) {
	rCategory := NewCategoryRepository(r.db, r.etcd)

	if category, err = rCategory.GetByName("default"); err != nil {
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
				return nil, err
			} else {
				log.Println("插入分类：", category)
				return category, nil
			}
		} else {
			return nil, err
		}
	} else {
		return category, nil
	}
}
