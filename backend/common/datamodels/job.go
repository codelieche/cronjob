package datamodels

import (
	"fmt"
	"log"
	"time"

	"github.com/gorhill/cronexpr"
)

// 定时任务
// 比如：每三十分钟执行一次的任务
// */30 * * * * echo `date` >> /var/log/test.log
type Job struct {
	BaseFields
	EtcdKey     string    `gorm:"size:100" json:"etcd_key, omitempty"`   // ETCD中保存的key
	Category    *Category `gorm:"ForeignKey:CategoryID" json:"category"` // Job的分类
	CategoryID  uint      `gorm:"INDEX;NOT NULL" json:"category_id"`     // 分类的ID
	Name        string    `gorm:"size:256" json:"name"`                  // 任务的名称
	Time        string    `gorm:"size:100;NOT NULL" json:"time"`         // 计划任务的时间
	Command     string    `gorm:"size:256;NOT NULL" json:"command"`      // 任务的命令
	Description string    `gorm:"size:512" json:"description,omitempty"` // Job描述
	IsActive    bool      `gorm:"type:boolean" json:"is_active"`         // 是否激活，激活才执行
	SaveOutput  bool      `gorm:"type:boolean" json:"save_output"`       // 是否记录输出
}

// 保存去Eetcd中的
type JobEtcd struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Category    string    `json:"category"`
	Name        string    `json:"name"`
	Time        string    `json:"time"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	SaveOutput  bool      `json:"save_output"`
}

// Job To JobEtcd
func (job *Job) ToEtcdStruct() *JobEtcd {
	return &JobEtcd{
		ID:          job.ID,
		CreatedAt:   job.CreatedAt,
		Category:    job.Category.Name,
		Name:        job.Name,
		Time:        job.Time,
		Command:     job.Command,
		Description: job.Description,
		IsActive:    job.IsActive,
		SaveOutput:  job.SaveOutput,
	}
}

// JobEtcd转换成JobExecutePlan
// 每次etcd监听到job的put操作的时候，就需要把Job信息转换成jobSchedulePlan
func (job *JobEtcd) ToJobExecutePlan() (jobSchedulePlan *JobSchedulePlan, err error) {
	var (
		expression *cronexpr.Expression
		now        time.Time
	)
	// 解析job的cron表达式
	if expression, err = cronexpr.Parse(job.Time); err != nil {
		msg := fmt.Sprintf("当前Job(ID:%d)解析时间表达式(%s)出错：%s", job.ID, job.Time, err.Error())
		log.Println(msg)
		return nil, err
	}

	// 生成job调度计划对象
	now = time.Now()
	jobSchedulePlan = &JobSchedulePlan{
		Job:        job,
		Expression: expression,
		NextTime:   expression.Next(now),
	}
	return jobSchedulePlan, nil
}
