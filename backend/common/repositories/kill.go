package repositories

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/jinzhu/gorm"
)

type JobKillRepository interface {
	// 保存JobKill
	Save(jobKill *datamodels.JobKill) (*datamodels.JobKill, error)
	// 获取JobKill
	Get(id int64) (jobKill *datamodels.JobKill, err error)
	// 获取Job Kill的列表
	List(offset int, limit int) (jobKills []*datamodels.JobKill, err error)
	// 设置JobKill为Finished
	SetFinishedByID(id int64) (jobKill *datamodels.JobKill, err error)
	// 修改JobKill
	Update(jobKill *datamodels.JobKill, fields map[string]interface{}) (*datamodels.JobKill, error)
	UpdateByID(id int64, fields map[string]interface{}) (*datamodels.JobKill, error)
}

func NewJobKillRepository(db *gorm.DB, etcd *datasources.Etcd) JobKillRepository {
	return &jobKillRepository{
		db:   db,
		etcd: etcd,
		infoFields: []string{
			"id", "created_at", "updated_at",
			"category", "job_id", "killed", "finished_at", "result",
		},
	}
}

type jobKillRepository struct {
	db         *gorm.DB
	etcd       *datasources.Etcd
	infoFields []string
}

func (r *jobKillRepository) Save(jobKill *datamodels.JobKill) (*datamodels.JobKill, error) {
	if jobKill.ID > 0 {
		// 是更新操作
		if err := r.db.Model(&datamodels.JobKill{}).Update(jobKill).Error; err != nil {
			log.Println("更新出错")
			return nil, err
		} else {
			// 判断是否需要从etcd中删除
			if jobKill.Killed {
				// 从etcd中删除
				if success, err := r.deleteJobKillFromEtcd(jobKill); err != nil {
					log.Println(err)
				} else {
					log.Println(success)
				}
			}
			return jobKill, nil
		}
	} else {
		// 是创建操作
		// 先检查这个job是否存在
		jobKillIsExist := &datamodels.JobKill{}
		if err := r.db.Select("id, category, job_id").
			First(&jobKillIsExist, "job_id = ? and killed = 0", jobKill.JobID).Error; err != nil {
			log.Println("查询jobKill的时候出错", err)
		} else {
			if jobKillIsExist.ID > 0 {
				// 存在
				log.Println("存在")
				err = fmt.Errorf("jobID=%d，已经存在未完成的jobKill(ID:%d)", jobKill.JobID, jobKillIsExist.ID)
				// 判断是否etcd中有
				if prevJobKill, err := r.getJobKillFromEtcd(jobKillIsExist.Category, jobKillIsExist.JobID); err != nil {
					log.Println(err)
					if err == common.NotFountError {
						// 需要插入到etcd中
						//log.Println(jobKill)
						if !jobKill.Killed {
							// 为killed就加入到etcd中
							if prevKillEtcd, err := r.saveJobkillToEtcd(jobKill, true); err != nil {
								log.Println("插入到etcd中，出错", err)
							} else {
								log.Println(prevKillEtcd)
							}
						}
					}
				} else {
					log.Println(prevJobKill)
				}
				return nil, err
				//return jobKillIsExist, nil
			}
		}

		// 开始创建JobKill
		if err := r.db.Create(jobKill).Error; err != nil {
			return nil, err
		} else {
			// 需要插入到etcd中
			//log.Println(jobKill)
			if !jobKill.Killed {
				// 为killed就加入到etcd中
				if prevKillEtcd, err := r.saveJobkillToEtcd(jobKill, true); err != nil {
					log.Println("插入到etcd中，出错", err)
				} else {
					log.Println(prevKillEtcd)
				}
			}
			return jobKill, nil
		}
	}
}

func (r *jobKillRepository) Get(id int64) (jobKill *datamodels.JobKill, err error) {
	jobKill = &datamodels.JobKill{}
	query := r.db.Select(r.infoFields).First(jobKill, "id = ?", id)

	if err = query.Error; err != nil {
		return nil, err
	} else {
		return jobKill, err
	}
}

func (r *jobKillRepository) List(offset int, limit int) (jobKills []*datamodels.JobKill, err error) {
	// DESC 降序，AESC 升序
	query := r.db.Model(&datamodels.JobKill{}).
		Select(r.infoFields).Order("killed ASC,ID DESC").
		Offset(offset).Limit(limit).Find(&jobKills)
	if err = query.Error; err != nil {
		return nil, err
	} else {
		return jobKills, nil
	}
}

// 设置为完成
func (r *jobKillRepository) SetFinishedByID(id int64) (jobKill *datamodels.JobKill, err error) {
	if jobKill, err = r.Get(id); err != nil {
		return nil, err
	} else {
		if jobKill.Killed {
			err = errors.New("当前JobKill已经是Killed")
			return jobKill, err
		} else {
			// 设置其为完成
			updatedFiled := make(map[string]interface{})
			updatedFiled["FinishedAt"] = time.Now()
			updatedFiled["Killed"] = true
			return r.Update(jobKill, updatedFiled)
		}
	}
}

func (r *jobKillRepository) Update(jobKill *datamodels.JobKill, fields map[string]interface{}) (*datamodels.JobKill, error) {
	// 判断ID：
	// 如果传入的是0，那么会更新全部
	// 如果fields中传入了ID，那么会更新ID是它的对象
	// 推荐加一个limit(1), 确保只更新一条数据
	if jobKill.ID <= 0 {
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
	if err := r.db.Model(jobKill).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		// 需要更新一下etcd中的数据
		if jobKill.Killed {
			// 需要从etcd中删除kill信息
			r.deleteJobKillFromEtcd(jobKill)
		} else {
			if prevEtcdJob, err := r.saveJobkillToEtcd(jobKill, false); err != nil {
				// 保存去etcd出错
				// 当不存在的时候，就需要重新创建一下
				if err == common.NotFountError {
					// 不存在etcd中，我们需要创建一下
					if _, err = r.saveJobkillToEtcd(jobKill, true); err != nil {
						log.Println("创建Job成功了，但是保存到etcd的时候，出错了", err.Error())
					}
				} else {
					log.Println("保存到mysql成功了，但是保存到etcd的时候，出错了", err.Error())
				}

			} else {
				//log.Println(prevEtcdJob)
				if prevEtcdJob == nil {
					log.Println("更新etcd没成功！")
				}
			}
		}
		return jobKill, nil
	}
}

func (r *jobKillRepository) UpdateByID(id int64, fields map[string]interface{}) (*datamodels.JobKill, error) {
	// 判断ID
	if id <= 0 {
		err := errors.New("传入的ID为0，会更新全部数据")
		return nil, err
	}

	// 更新操作
	//if err := r.db.Model(&datamodels.JobKill{}).Where("id = ?", id).Limit(1).Update(fields).Error; err != nil {
	//	return nil, err
	//} else {
	//	// 返回获取到的对象
	//	return r.Get(id)
	//}
	if jobKill, err := r.Get(id); err != nil {
		return nil, err
	} else {
		return r.Update(jobKill, fields)
	}
}
