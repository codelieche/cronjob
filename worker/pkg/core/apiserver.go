package core

type ApiserverResponse struct {
	Code    int         `json:"code"`    // 返回的code，如果是0就表示正常
	Message string      `json:"message"` // 返回的消息
	Data    interface{} `json:"data"`    // 返回的数据
}

type Apiserver interface {
	GetCategory(category string) (*Category, error) // 获取任务分类
	GetTask(taskID string) (*Task, error)           // 获取任务详情
}
