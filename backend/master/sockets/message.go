package sockets

// 客户端发送的消息
type Message struct {
	RemoteAddr string // 客户端的地址
	Data       []byte // 接收到的消息内容
}

// 消息事件
// 通过消息来判断事件的类型，比如:message, createJob, jobExecute, tryLock, leaseLock, releaseLock,
type MessageEvent struct {
	Category string `json:"category"` // 消息分类
	Data     string `json:"data"`     // 数据
}

// 消息处理函数
