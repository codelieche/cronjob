package common

// 定时任务
// 比如：每三十分钟执行一次的任务
// */30 * * * * echo `date` >> /var/log/test.log
type Job struct {
	Name        string `json:"name"`                  // 任务的名称
	Time        string `json:"time"`                  // 计划任务的时间
	Command     string `json:"command"`               // 任务的命令
	Description string `json:"description,omitempty"` // Job描述
}
