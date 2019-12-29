package datamodels

// LockRequest
type LockRequest struct {
	ID     int    `json:"id"`     // 发起上锁请求的序号
	Name   string `json:"name"`   // 锁的名字
	Secret string `json:"secret"` // 锁的秘钥
}

// 通过http获取锁的响应结果
type LockResponse struct {
	ID      int    `json:"id"`      // 发起上锁请求的序号
	Success bool   `json:"success"` // 是否成功
	Name    string `json:"name"`    // 锁的名字
	Secret  string `json:"secret"`  // 锁的秘钥
	Message string `json:"message"` // 消息内容
}

// 通用的响应消息
type BaseResponse struct {
	Status  string `json:"status"`  // 状态
	Message string `json:"message"` // 消息内容
}
