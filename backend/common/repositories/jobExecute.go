package repositories

import (
	"errors"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/jinzhu/gorm"
)

type JobExecuteRepository interface {
	// 创建JobExecute
	Create(jobExecute *datamodels.JobExecute) (*datamodels.JobExecute, error)
	// 根据ID获取JobExecute
	Get(id int64) (jobExecute *datamodels.JobExecute, err error)
	// 获取JobExecute的列表
	List(offset int, limit int) (jobExecutes []*datamodels.JobExecute, err error)
	// 更新
	Update(jobExecute *datamodels.JobExecute, fields map[string]interface{}) (*datamodels.JobExecute, error)
	// 根据ID更新
	UpdateByID(id int64, fields map[string]interface{}) (jobExecute *datamodels.JobExecute, err error)

	// 回写执行结果信息
	SaveExecuteLog(jobExecuteResult *datamodels.JobExecuteResult) (jobExecute *datamodels.JobExecute, err error)

	// 获取JobExecute的Log
	GetExecuteLog(jobExecute *datamodels.JobExecute) (jobExecuteLog *datamodels.JobExecuteLog, err error)
	GetExecuteLogByID(id int64) (jobExecuteLog *datamodels.JobExecuteLog, err error)
}

func NewJobExecuteRepository(db *gorm.DB) JobExecuteRepository {
	return &jobExecuteRepository{
		db: db,
		infoFields: []string{
			"id", "created_at", "updated_at",
			"category", "name", "job_id", "command",
			"status", "plan_time", "schedule_time", "start_time", "end_time",
		},
	}
}

type jobExecuteRepository struct {
	db         *gorm.DB
	infoFields []string
}

func (r *jobExecuteRepository) Create(jobExecute *datamodels.JobExecute) (*datamodels.JobExecute, error) {
	// 判断是否有ID
	if jobExecute.ID > 0 {
		err := errors.New("不可创建设置了ID的对象")
		return nil, err
	} else {
		// 创建操作
		if err := r.db.Create(jobExecute).Error; err != nil {
			return nil, err
		} else {
			return jobExecute, nil
		}
	}
}

func (r *jobExecuteRepository) Get(id int64) (jobExecute *datamodels.JobExecute, err error) {
	jobExecute = &datamodels.JobExecute{}
	if err = r.db.Select(r.infoFields).First(jobExecute, "id = ?", id).Error; err != nil {
		return nil, err
	} else {
		if jobExecute.ID > 0 {
			return jobExecute, nil
		} else {
			return nil, common.NotFountError
		}
	}
}

func (r *jobExecuteRepository) List(offset int, limit int) (jobExecutes []*datamodels.JobExecute, err error) {
	query := r.db.Model(&datamodels.JobExecute{}).Select(r.infoFields).Offset(offset).Limit(limit).Find(&jobExecutes)

	if err = query.Error; err != nil {
		return nil, err
	} else {
		return jobExecutes, nil
	}

}

func (r *jobExecuteRepository) Update(jobExecute *datamodels.JobExecute, fields map[string]interface{}) (*datamodels.JobExecute, error) {
	// 判断ID：
	// 如果传入的是0，那么会更新全部
	// 如果fields中传入了ID，那么会更新ID是它的对象
	// 推荐加一个limit(1), 确保只更新一条数据
	if jobExecute.ID <= 0 {
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
	if err := r.db.Model(jobExecute).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		return jobExecute, nil
	}
}

func (r *jobExecuteRepository) UpdateByID(id int64, fields map[string]interface{}) (jobExecute *datamodels.JobExecute, err error) {
	// 判断ID
	if id <= 0 {
		err := errors.New("传入的ID为0，会更新全部数据")
		return nil, err
	}

	if jobExecute, err = r.Get(id); err != nil {
		return nil, err
	} else {
		return r.Update(jobExecute, fields)
	}
}

func (r *jobExecuteRepository) SaveExecuteLog(jobExecuteResult *datamodels.JobExecuteResult) (jobExecute *datamodels.JobExecute, err error) {
	panic("implement me")
}

func (r *jobExecuteRepository) GetExecuteLog(jobExecute *datamodels.JobExecute) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	panic("implement me")
}

func (r *jobExecuteRepository) GetExecuteLogByID(id int64) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	panic("implement me")
}
