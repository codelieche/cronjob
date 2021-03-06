package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gopkg.in/mgo.v2/bson"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/codelieche/cronjob/backend/common/datasources"

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
	// Kill Job Execute
	KillByID(id int64) (success bool, err error)
}

func NewJobExecuteRepository(db *gorm.DB, etcd *datasources.Etcd, mongoDB *datasources.MongoDB) JobExecuteRepository {

	return &jobExecuteRepository{
		db:      db,
		etcd:    etcd,
		mongoDB: mongoDB,
		infoFields: []string{
			"id", "created_at", "updated_at",
			"worker", "category", "name", "job_id", "command",
			"status", "plan_time", "schedule_time", "start_time", "end_time", "log_id",
		},
	}
}

type jobExecuteRepository struct {
	db         *gorm.DB
	mongoDB    *datasources.MongoDB
	etcd       *datasources.Etcd
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
	// 保存执行日志
	var (
		errStr  string
		success bool
		status  string
	)
	// 状态可能是：kill、timeout
	status = jobExecuteResult.Status
	if jobExecuteResult.Error != "" {
		errStr = jobExecuteResult.Error
		success = false
		if status == "" {
			status = "error"
		}
	} else {
		success = true
		if status == "" {
			status = "done"
		}
	}
	jobExecuteLog := &datamodels.JobExecuteLog{
		JobExecuteID: jobExecuteResult.ExecuteID,
		Output:       string(jobExecuteResult.Output),
		Error:        errStr,
		Success:      success,
	}
	// 插入到mongo中
	if insertOneResult, err := r.mongoDB.Collection.InsertOne(context.TODO(), jobExecuteLog); err != nil {
		log.Println(err.Error())
	} else {
		//log.Println("插入数据成功", insertOneResult.InsertedID)
		objectID := insertOneResult.InsertedID.(primitive.ObjectID)
		//log.Println(objectID.Hex())

		updateFields := make(map[string]interface{})
		updateFields["log_id"] = objectID.Hex()
		updateFields["status"] = status
		updateFields["EndTime"] = time.Now()
		return r.UpdateByID(int64(jobExecuteResult.ExecuteID), updateFields)
	}
	return
}

// 获取JobExecute的执行日志
// 这个是保存在MongoDB中的，先得到ObjectID，再获取对象
func (r *jobExecuteRepository) GetExecuteLog(jobExecute *datamodels.JobExecute) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	if jobExecute.LogID == "" {
		//log.Println("LogID为空")
		return nil, common.NotFountError
	} else {
		// 从mongo中获取执行结果
		objectID, err := primitive.ObjectIDFromHex(jobExecute.LogID)
		if err != nil {
			//log.Println(err)
			// ObjectID无效
			return nil, err
		} else {
			filter := bson.M{"_id": objectID}
			//filter = bson.M{"job_execute_id": jobExecute.ID}
			if err = r.mongoDB.Collection.FindOne(context.Background(), filter).Decode(&jobExecuteLog); err != nil {
				log.Println(err)
				return nil, err
			} else {
				return jobExecuteLog, err
			}
		}
	}
}

func (r *jobExecuteRepository) GetExecuteLogByID(id int64) (jobExecuteLog *datamodels.JobExecuteLog, err error) {
	if jobExecute, err := r.Get(id); err != nil {
		return nil, err
	} else {
		return r.GetExecuteLog(jobExecute)
	}
}

// Kill Job Execute
func (r *jobExecuteRepository) KillByID(id int64) (success bool, err error) {
	// 1. 定义变量
	var (
		jobExecute    *datamodels.JobExecute
		jobKill       *datamodels.JobKill
		etcdKey       string
		etcdValueData []byte
		etcdValue     string
	)

	// 2. 获取jobExecute
	// 2-1：获取
	if jobExecute, err = r.Get(id); err != nil {
		return false, err
	} else {
	}

	// 2-2：判断执行的状态
	if jobExecute.Status != "start" && jobExecute.Status != "todo" && jobExecute.Status != "doding" {
		err = fmt.Errorf("当前执行的状态是%s，不可kill了", jobExecute.Status)
		return false, err
	}

	// 3. 创建jobKill到etcd中
	// 3-1: 创建jobKill
	jobKill = &datamodels.JobKill{
		Category: jobExecute.Category,
		JobID:    uint(jobExecute.JobID),
		Killed:   false,
	}
	// 3-2: json序列化
	if etcdValueData, err = json.Marshal(jobKill); err != nil {
		return false, err
	}

	etcdValue = string(etcdValueData)
	// 3-3: 插入到etcd中
	etcdKey = fmt.Sprintf("%s%s/%d", common.ETCD_JOB_KILL_DIR, jobKill.Category, jobKill.JobID)
	if _, err = r.etcd.PutKeyValue(etcdKey, etcdValue); err != nil {
		return false, err
	} else {
		return true, nil
	}
}
