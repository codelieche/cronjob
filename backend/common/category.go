package common

// 计划任务的分类：默认是Default
// 执行不同类型的计划任务，需要不同的环境准备
// 环境准备可以通过Command来处理
// 当worker启动的时候：会添加default的分类
// 执行分类相关的命令：
// 1. 先执行checkCmd的命令：成功就跳过，
// 2. 未成功就执行SetupCmd，再执行CheckCmd
// 3. 当worker关闭的时候，执行TearDownCmd的命令。
type Category struct {
	IsActive    bool   `json:"is_active"`     // 分类的状态，如果是true才可以执行
	Key         string `json:"key"`           // etcd中保存的key
	Name        string `json:"name"`          // 分类名称
	Description string `json:"description"`   // 分类的描述信息
	CheckCmd    string `json:"check_cmd""`    // 检查是否可以执行本来计划任务的命令：eg：ls `which bash`
	SetupCmd    string `json:"setup_cmd"`     // worker节点初始化的时候执行的命令, eg: pip install requests
	TearDownCmd string `json:"tear_down_cmd"` // worker节点退出的时候需要执行的命令, eg: pip uninstall requests
}

// 分类执行会用到的命令
type CategoryCommand struct {
	Check    string `json:"check"`     // 检查是否可以执行本来计划任务的命令：eg：ls `which bash`
	Setup    string `json:"setup"`     // worker节点初始化的时候执行的命令, eg: pip install requests
	TearDown string `json:"tear_down"` // worker节点退出的时候需要执行的命令, eg: pip uninstall requests
}
