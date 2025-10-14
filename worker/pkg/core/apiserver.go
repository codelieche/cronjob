package core

type ApiserverResponse struct {
	Code    int         `json:"code"`    // 返回的code，如果是0就表示正常
	Message string      `json:"message"` // 返回的消息
	Data    interface{} `json:"data"`    // 返回的数据
}

type Apiserver interface {
	GetCategory(category string) (*Category, error)                            // 获取任务分类
	GetTask(taskID string) (*Task, error)                                      // 获取任务详情
	AppendTaskLog(taskID string, content string) error                         // 追加/创建任务日志
	AcquireLock(key string, expire int) (lockKey, lockValue string, err error) // 获取分布式锁，返回key和value
	PingWorker(workerID string) error                                          // 发送Worker心跳，更新is_active状态
	GetCredential(credentialID string) (*Credential, error)                    // 🔥 获取凭证（已解密的明文）
}
