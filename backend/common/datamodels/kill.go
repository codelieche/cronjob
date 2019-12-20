package datamodels

import "time"

type JobKill struct {
	BaseFields
	EtcdKey    string     `gorm:"size:100" json:"etcd_key"`     // 保存在Etcd中的Key
	Category   string     `gorm:"size:100" json:"category"`     // 要杀掉的Job的分类
	JobID      uint       `gorm:"INDEX;NOT NULL" json:"job_id"` // 杀掉的Job的ID
	Killed     bool       `gorm:"type:boolean" json:"killed"`   // 是否已经杀死
	FinishedAt *time.Time `gorm:"NULL" json:"finished_at"`      // 完成时间
	Result     string     `gorm:"size:512" json:"result"`       // 处理结果
}

type KillEtcd struct {
	ID        uint      `json:"id"`         // ID
	CreatedAt time.Time `json:"created_at"` // 创建时间
	Category  string    `json:"category"`   // Job的分类名
	JobID     uint      `json:"job_id"`     // 要杀掉的Job的ID
	Killed    bool      `json:"killed"`     //是否已经执行了Kill
}

func (k *JobKill) ToEtcdDataStruct() *KillEtcd {
	return &KillEtcd{
		ID:        k.ID,
		CreatedAt: k.CreatedAt,
		Category:  k.Category,
		JobID:     k.JobID,
		Killed:    k.Killed,
	}
}
