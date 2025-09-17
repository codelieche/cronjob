package core

import (
	"context"
	"encoding/json"
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

// ========== WebSocket客户端相关接口 ==========

// WebsocketClient 定义WebSocket客户端连接接口
type WebsocketClient interface {
	ID() string                  // 获取客户端唯一标识
	Send(event *TaskEvent) error // 发送任务事件到客户端
	Close()                      // 关闭客户端连接
}

// WebsocketClientManager 定义客户端管理器接口
type WebsocketClientManager interface {
	Add(client WebsocketClient)                   // 添加客户端
	Remove(clientID string)                       // 移除指定ID的客户端
	Broadcast(event *TaskEvent)                   // 广播任务事件给所有客户端
	Count() int                                   // 获取当前连接的客户端数量
	RegistWorker(clientID string, worker *Worker) // 注册Worker信息
	GetWorkers() map[string]*Worker               // 获取所有Worker信息
}

// ========== WebSocket服务接口 ==========

// WebsocketService 定义WebSocket服务接口
type WebsocketService interface {
	// 任务事件处理相关
	HandleTaskEvent() error                               // 处理任务事件
	StartConsumingQueues() error                          // 开始消费任务队列
	GetClientManager() WebsocketClientManager             // 获取客户端管理器
	GetPendingTasks(ctx context.Context) ([]*Task, error) // 获取待执行任务

	// Worker相关方法
	GetWorkerByID(ctx context.Context, id string) (*Worker, error)     // 根据ID获取Worker
	UpdateWorker(ctx context.Context, worker *Worker) (*Worker, error) // 更新Worker信息
	CreateWorker(ctx context.Context, worker *Worker) (*Worker, error) // 创建Worker

	// Task相关方法
	GetTaskByID(ctx context.Context, id string) (*Task, error)                             // 根据ID获取Task
	UpdateTaskFields(ctx context.Context, id string, updates map[string]interface{}) error // 更新Task的特定字段
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
}
