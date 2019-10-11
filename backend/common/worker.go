package common

// Worker节点的信息
type WorkerInfo struct {
	Name string `json:"name"` // 节点的名称：Ip:Port（这样就算唯一的了）
	Host string `json:"host"` // 主机名
	User string `json:"user"` // 执行程序的用户
	Ip   string `json:"ip"`   // IP地址
	Port int    `json:"port"` // worker 监控服务的端口
	Pid  int    `json:"pid"`  // Worker的端口号
}
