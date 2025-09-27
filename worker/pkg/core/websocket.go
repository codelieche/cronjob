package core

import (
	"encoding/json"
	"time"
)

// ========== 服务端到客户端的任务事件相关定义 ==========

// TaskAction 定义任务动作类型常量
type TaskAction string

const (
	TaskActionRun     TaskAction = "run"     // 运行任务
	TaskActionStop    TaskAction = "stop"    // 停止任务
	TaskActionKill    TaskAction = "kill"    // 强制终止任务
	TaskActionTimeout TaskAction = "timeout" // 任务超时
	TaskActionRetry   TaskAction = "retry"   // 重试任务
)

// TaskEvent 定义任务事件结构
type TaskEvent struct {
	Action string  `json:"action"` // 任务动作类型
	Tasks  []*Task `json:"tasks"`  // 任务列表
}

// ========== 客户端到服务端的事件相关定义 ==========

// 客户端事件动作类型常量
const (
	ClientEventActionPing         = "ping"          // Ping通信，维持WebSocket连接
	ClientEventActionTaskUpdate   = "task_update"   // 更新任务信息
	ClientEventActionRegistWorker = "regist_worker" // 注册Worker信息
)

// ClientEvent 定义客户端发送到服务端的事件结构
type ClientEvent struct {
	Action   string          `json:"action"`    // 操作类型
	WorkerID string          `json:"worker_id"` // 工作节点ID
	TaskID   string          `json:"task_id"`   // 任务ID
	Data     json.RawMessage `json:"data"`      // 附加数据
	ApiKey   string          `json:"api_key"`   // API Key
}

// ========== WebSocket服务接口定义 ==========

// WebsocketService WebSocket服务接口
// 定义了WebSocket连接、消息处理等核心功能
type WebsocketService interface {
	// 连接管理
	Connect() error    // 连接到apiserver的WebSocket
	Close()            // 关闭WebSocket连接
	IsConnected() bool // 检查连接状态

	Setup() error    // 设置WebSocket服务
	Teardown() error // 卸载WebSocket服务

	// 消息发送
	SendPing() error                                                 // 发送心跳ping消息
	SendTaskUpdate(taskID string, data map[string]interface{}) error // 发送任务更新消息

	// 消息处理
	HandleMessage(message []byte) // 处理收到的消息

	// 生命周期管理
	Start() error // 启动WebSocket服务
	Stop()        // 停止WebSocket服务
}

// ========== Task服务接口定义 ==========

// TaskService 任务服务接口
// 定义了任务执行、管理等功能
// 继承TaskEventHandler接口，用于处理任务事件
type TaskService interface {
	TaskEventHandler // 继承任务事件处理接口

	// 任务执行
	ExecuteTask(task *Task) // 执行任务

	// 任务操作
	RunTasks(tasks []*Task)     // 运行任务列表
	StopTasks(tasks []*Task)    // 停止任务列表
	KillTasks(tasks []*Task)    // 强制终止任务列表
	TimeoutTasks(tasks []*Task) // 处理超时任务列表
	RetryTasks(tasks []*Task)   // 重试任务列表

	// 状态查询（用于优雅关闭）
	GetRunningTaskCount() int                           // 获取正在运行的任务数量
	GetRunningTaskIDs() []string                        // 获取正在运行的任务ID列表
	WaitForTasksCompletion(timeout time.Duration) error // 等待所有任务完成
}

// ========== WebSocket配置结构 ==========

// WebsocketConfig WebSocket配置
type WebsocketConfig struct {
	ServerURL        string        `json:"server_url"`        // 服务器URL
	PingInterval     time.Duration `json:"ping_interval"`     // Ping间隔
	ReconnectDelay   time.Duration `json:"reconnect_delay"`   // 重连延迟
	ReadTimeout      time.Duration `json:"read_timeout"`      // 读取超时
	WriteTimeout     time.Duration `json:"write_timeout"`     // 写入超时
	MessageSeparator string        `json:"message_separator"` // 消息分隔符
	MaxMessageSize   int           `json:"max_message_size"`  // 最大消息大小（字节）
}

// DefaultWebsocketConfig 返回默认的WebSocket配置
func DefaultWebsocketConfig() *WebsocketConfig {
	return &WebsocketConfig{
		PingInterval:     20 * time.Second, // 增加ping间隔到20秒
		ReconnectDelay:   5 * time.Second,
		ReadTimeout:      90 * time.Second, // 增加读取超时到90秒，ping间隔的3倍
		WriteTimeout:     30 * time.Second,
		MessageSeparator: "\x00223399AABB2233CC",
		MaxMessageSize:   102400,
	}
}
