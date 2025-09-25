package services

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocket服务相关常量定义
const (
	// 发送消息的最大任务数量
	MaxTasksPerMessage = config.WebsocketMaxTasksPerMessage
	// 消息分隔符 - 用于在WebSocket消息中分隔不同的事件
	MessageSeparator = config.WebsocketMessageSeparator
)

// ========== WebSocket客户端实现 ==========

// ClientImpl 实现了core.WebsocketClient接口
// 负责管理单个WebSocket客户端连接的生命周期和消息发送

type ClientImpl struct {
	id       string               // 客户端唯一标识
	conn     *websocket.Conn      // WebSocket连接实例
	sendChan chan *core.TaskEvent // 消息发送通道，缓冲大小为100
	doneChan chan struct{}        // 关闭信号通道
	closed   bool                 // 连接关闭状态
	mutex    sync.Mutex           // 互斥锁，保护closed状态
}

// NewClient 创建WebSocket客户端实例
// 参数:
//   - id: 客户端唯一标识
//   - conn: WebSocket连接对象
//
// 返回值:
//   - *ClientImpl: 客户端实例指针
func NewClient(id string, conn *websocket.Conn) *ClientImpl {
	client := &ClientImpl{
		id:       id,
		conn:     conn,
		sendChan: make(chan *core.TaskEvent, 100),
		doneChan: make(chan struct{}),
	}

	// 启动发送消息的goroutine
	go client.writePump()

	return client
}

// ID 返回客户端唯一标识
func (c *ClientImpl) ID() string {
	return c.id
}

// Send 发送任务事件到客户端
// 参数:
//   - event: 要发送的任务事件对象
//
// 返回值:
//   - error: 发送过程中的错误，如通道满等
func (c *ClientImpl) Send(event *core.TaskEvent) error {
	select {
	case c.sendChan <- event:
		return nil
	default:
		logger.Warn("客户端发送通道已满", zap.String("client_id", c.id))
		return nil
	}
}

// Close 安全关闭客户端连接
func (c *ClientImpl) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.closed {
		close(c.doneChan)
		c.conn.Close()
		c.closed = true
	}
}

// writePump 持续从发送通道获取消息并发送到WebSocket连接
// 这个goroutine确保消息能够异步地发送到客户端
func (c *ClientImpl) writePump() {
	defer func() {
		c.Close()
	}()

	for {
		select {
		case <-c.doneChan:
			return
		case event := <-c.sendChan:
			// 序列化事件为JSON
			data, err := json.Marshal(event)
			if err != nil {
				logger.Error("序列化TaskEvent失败", zap.Error(err))
				continue
			}

			// 添加消息分隔符，前后都添加确保接收端能正确解析
			dataWithSeparator := []byte(MessageSeparator)
			dataWithSeparator = append(dataWithSeparator, data...)
			dataWithSeparator = append(dataWithSeparator, []byte(MessageSeparator)...)

			// 通过WebSocket发送文本消息
			if err := c.conn.WriteMessage(websocket.TextMessage, dataWithSeparator); err != nil {
				logger.Error("发送WebSocket消息失败", zap.Error(err), zap.String("client_id", c.id))
				return
			}
		}
	}
}

// ========== WebSocket客户端管理器实现 ==========

// ClientManagerImpl 实现了core.WebsocketClientManager接口
// 负责管理所有连接的WebSocket客户端，并提供广播功能

type ClientManagerImpl struct {
	clients     map[string]core.WebsocketClient // 存储所有客户端连接
	mutex       sync.RWMutex                    // 读写锁，保护clients集合
	workers     map[string]*core.Worker         // 存储所有客户端Worker的信息
	mutexWorker sync.RWMutex                    // 读写worker信息
}

// 全局客户端管理器实例
var clientManager = NewClientManagerInstance()

// NewClientManagerInstance 创建一个新的客户端管理器内部实例
func NewClientManagerInstance() *ClientManagerImpl {
	return &ClientManagerImpl{
		clients: make(map[string]core.WebsocketClient),
		workers: make(map[string]*core.Worker),
	}
}

// NewClientManager 获取全局客户端管理器实例
func NewClientManager() *ClientManagerImpl {
	return clientManager
}

// Add 添加一个客户端到管理器中
// 参数:
//   - client: 要添加的WebSocket客户端对象
func (cm *ClientManagerImpl) Add(client core.WebsocketClient) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.clients[client.ID()] = client
	logger.Info("客户端已连接", zap.String("client_id", client.ID()), zap.Int("total_clients", len(cm.clients)))
}

// Remove 从管理器中移除指定ID的客户端并关闭连接
// 参数:
//   - clientID: 要移除的客户端唯一标识
func (cm *ClientManagerImpl) Remove(clientID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	if client, exists := cm.clients[clientID]; exists {
		client.Close()
		delete(cm.clients, clientID)
		logger.Info("客户端已断开连接", zap.String("client_id", clientID), zap.Int("total_clients", len(cm.clients)))
		// 同时移除对应的worker信息
		cm.mutexWorker.Lock()
		tmpClientID := clientID // 创建临时变量供defer使用
		defer func() {
			cm.mutexWorker.Unlock()
			logger.Info("Worker信息已移除", zap.String("client_id", tmpClientID))
		}()
		delete(cm.workers, clientID)
	}
}

// RegistWorker 注册客户端的Worker信息
// 参数:
//   - clientID: 客户端唯一标识
//   - worker: 客户端对应的Worker对象
func (cm *ClientManagerImpl) RegistWorker(clientID string, worker *core.Worker) {
	cm.mutexWorker.Lock()
	defer cm.mutexWorker.Unlock()
	cm.workers[clientID] = worker
}

func (cm *ClientManagerImpl) GetWorkers() map[string]*core.Worker {
	cm.mutexWorker.Lock()
	defer cm.mutexWorker.Unlock()
	return cm.workers
}

// Broadcast 广播任务事件给所有连接的客户端
// 参数:
//   - event: 要广播的任务事件对象
func (cm *ClientManagerImpl) Broadcast(event *core.TaskEvent) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, client := range cm.clients {
		clientID := client.ID()

		// 获取客户端对应的Worker信息
		var supportedTasks []string
		var workerName string
		cm.mutexWorker.RLock()
		worker, exists := cm.workers[clientID]
		cm.mutexWorker.RUnlock()

		// 如果存在Worker信息，尝试解析其支持的任务类型
		if exists && worker != nil {
			workerName = worker.Name
			if worker.Metadata != nil {
				// 定义一个临时结构来解析Metadata中的tasks字段
				var metadata core.WorkerMetadata

				// 尝试解析Metadata
				if err := json.Unmarshal(worker.Metadata, &metadata); err == nil {
					supportedTasks = metadata.Tasks // 支持的任务类型列表
				}
			}
		}

		// 如果没有指定支持的任务类型或者任务列表为空，直接发送消息
		if len(supportedTasks) == 0 || len(event.Tasks) == 0 {
			if err := client.Send(event); err != nil {
				logger.Error("广播消息失败", zap.Error(err), zap.String("client_id", clientID))
			}
			continue
		}

		// 创建一个过滤后的任务事件
		filteredEvent := &core.TaskEvent{
			Action: event.Action,
			Tasks:  []*core.Task{},
		}

		// 根据Worker支持的任务类型和WorkerSelect过滤任务
		for _, task := range event.Tasks {
			// 首先检查任务的Category是否在Worker支持的任务类型列表中
			categoryMatched := false
			for _, supportedTask := range supportedTasks {
				if task.Category == supportedTask {
					categoryMatched = true
					break
				}
			}

			if !categoryMatched {
				continue
			}

			// 检查任务的WorkerSelect配置
			if len(task.Metadata) > 0 {
				taskMetadata, err := task.GetMetadata()
				if err != nil {
					logger.Warn("解析任务元数据失败", zap.Error(err), zap.String("task_id", task.ID.String()))
					// 解析失败时，仍然按照原逻辑处理
					filteredEvent.Tasks = append(filteredEvent.Tasks, task)
					continue
				}

				// 如果任务指定了WorkerSelect，检查当前Worker是否在列表中
				if len(taskMetadata.WorkerSelect) > 0 {
					workerSelected := false
					for _, selectedWorker := range taskMetadata.WorkerSelect {
						// 支持按Worker ID或Name进行匹配
						if selectedWorker == clientID || selectedWorker == workerName || (worker != nil && selectedWorker == worker.ID.String()) {
							workerSelected = true
							break
						}
					}

					// 如果当前Worker不在选择列表中，跳过这个任务
					if !workerSelected {
						logger.Debug("任务指定了WorkerSelect，当前Worker不在选择列表中",
							zap.String("client_id", clientID),
							zap.String("worker_name", workerName),
							zap.String("task_id", task.ID.String()),
							zap.Strings("worker_select", taskMetadata.WorkerSelect))
						continue
					}
				}
			}

			// 通过所有过滤条件，添加到过滤后的任务列表
			filteredEvent.Tasks = append(filteredEvent.Tasks, task)
		}

		// 只有当过滤后的任务列表长度大于0时，才发送消息
		if len(filteredEvent.Tasks) > 0 {
			if err := client.Send(filteredEvent); err != nil {
				logger.Error("广播消息失败", zap.Error(err), zap.String("client_id", clientID))
			} else {
				logger.Debug("广播过滤后的任务消息", zap.String("client_id", clientID),
					zap.Int("original_tasks", len(event.Tasks)),
					zap.Int("filtered_tasks", len(filteredEvent.Tasks)))
			}
		} else {
			logger.Debug("没有符合Worker条件的任务，不发送消息",
				zap.String("client_id", clientID),
				zap.String("worker_name", workerName),
				zap.Strings("supported_tasks", supportedTasks))
		}
	}
}

// Count 获取当前连接的客户端数量
// 返回值:
//   - int: 客户端数量
func (cm *ClientManagerImpl) Count() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return len(cm.clients)
}

// ========== WebSocket服务实现 ==========

// WebsocketService 实现了core.WebsocketService接口
// 提供WebSocket相关的业务逻辑，包括任务事件处理、队列消费等

type WebsocketService struct {
	taskStore     core.TaskStore     // 任务数据存储接口
	workerStore   core.WorkerStore   // Worker数据存储接口
	clientManager *ClientManagerImpl // 客户端管理器实例
}

// NewWebsocketService 创建WebSocket服务实例
// 参数:
//   - taskStore: 任务数据存储接口
//   - workerStore: Worker数据存储接口
//
// 返回值:
//   - core.WebsocketService: WebSocket服务接口
func NewWebsocketService(taskStore core.TaskStore, workerStore core.WorkerStore) core.WebsocketService {
	return &WebsocketService{
		taskStore:     taskStore,
		workerStore:   workerStore,
		clientManager: NewClientManager(),
	}
}

// HandleTaskEvent 处理任务事件
// 目前暂未实现具体逻辑
// 返回值:
//   - error: 处理过程中的错误
func (w *WebsocketService) HandleTaskEvent() error {
	// 这个方法可以用于处理任务事件，暂时返回nil
	return nil
}

// GetWorkerByID 根据ID获取Worker信息
// 参数:
//   - ctx: 请求上下文
//   - id: Worker的ID字符串
//
// 返回值:
//   - *core.Worker: Worker对象指针
//   - error: 查询过程中的错误
func (w *WebsocketService) GetWorkerByID(ctx context.Context, id string) (*core.Worker, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析Worker ID失败", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return w.workerStore.FindByID(ctx, uuidID)
}

// UpdateWorker 更新Worker信息
// 参数:
//   - ctx: 请求上下文
//   - worker: 包含更新信息的Worker对象
//
// 返回值:
//   - *core.Worker: 更新后的Worker对象指针
//   - error: 更新过程中的错误
func (w *WebsocketService) UpdateWorker(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	return w.workerStore.Update(ctx, worker)
}

// CreateWorker 创建新的Worker
// 参数:
//   - ctx: 请求上下文
//   - worker: 要创建的Worker对象
//
// 返回值:
//   - *core.Worker: 创建后的Worker对象指针
//   - error: 创建过程中的错误
func (w *WebsocketService) CreateWorker(ctx context.Context, worker *core.Worker) (*core.Worker, error) {
	return w.workerStore.Create(ctx, worker)
}

// GetTaskByID 根据ID获取Task信息
// 参数:
//   - ctx: 请求上下文
//   - id: Task的ID字符串
//
// 返回值:
//   - *core.Task: Task对象指针
//   - error: 查询过程中的错误
func (w *WebsocketService) GetTaskByID(ctx context.Context, id string) (*core.Task, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析Task ID失败", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return w.taskStore.FindByID(ctx, uuidID)
}

// UpdateTaskFields 部分更新Task的特定字段
// 参数:
//   - ctx: 请求上下文
//   - id: Task的ID字符串
//   - updates: 包含要更新字段和值的映射
//
// 返回值:
//   - error: 更新过程中的错误
func (w *WebsocketService) UpdateTaskFields(ctx context.Context, id string, updates map[string]interface{}) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("解析Task ID失败", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	return w.taskStore.Patch(ctx, uuidID, updates)
}

// StartConsumingQueues 启动队列消费goroutines
// 同时启动待执行任务队列和停止任务队列的消费者
// 返回值:
//   - error: 启动过程中的错误
func (w *WebsocketService) StartConsumingQueues() error {
	logger.Debug("开始启动队列消费者")
	go w.consumePendingTasksQueue()
	go w.consumeStopTasksQueue()
	logger.Debug("队列消费者启动完成")

	return nil
}

// GetClientManager 获取客户端管理器实例
// 返回值:
//   - core.WebsocketClientManager: 客户端管理器接口
func (w *WebsocketService) GetClientManager() core.WebsocketClientManager {
	return w.clientManager
}

// consumePendingTasksQueue 消费待执行任务队列
// 这个goroutine会持续从待执行队列中获取任务，并广播给所有连接的客户端
func (w *WebsocketService) consumePendingTasksQueue() {
	logger.Debug("启动待执行任务队列消费者")

	for {
		logger.Info("开始消费待执行任务队列")
		time.Sleep(time.Second)

		for task := range GetPendingTasksQueue() {
			clientCount := w.clientManager.Count()
			if clientCount > 0 {
				// 有客户端连接，广播任务
				event := &core.TaskEvent{
					Action: string(core.TaskActionRun),
					Tasks:  []*core.Task{task},
				}
				w.clientManager.Broadcast(event)
			} else {
				// 没有客户端，检查任务是否已超时
				now := time.Now()
				if task.TimeoutAt.After(now) {
					// 任务未超时，重新放回队列
					select {
					case pendingTasksQueue <- task:
						logger.Debug("任务重新放回待执行队列", zap.String("task_id", task.ID.String()))
					default:
						logger.Warn("待执行队列已满，无法重新放回任务", zap.String("task_id", task.ID.String()))
					}
				}
			}
		}
		time.Sleep(time.Second)
	}
}

// consumeStopTasksQueue 消费停止任务队列
// 这个goroutine会持续从停止队列中获取任务，并广播给所有连接的客户端
func (w *WebsocketService) consumeStopTasksQueue() {
	logger.Info("启动停止任务队列消费者")
	for task := range GetStopTasksQueue() {
		clientCount := w.clientManager.Count()
		if clientCount > 0 {
			// 有客户端连接，广播停止任务
			event := &core.TaskEvent{
				Action: string(core.TaskActionStop),
				Tasks:  []*core.Task{task},
			}
			w.clientManager.Broadcast(event)
		} else {
			// 没有客户端，检查任务是否已超时
			now := time.Now()
			if task.TimeoutAt.After(now) {
				// 任务未超时，重新放回队列
				select {
				case stopTasksQueue <- task:
					logger.Debug("任务重新放回停止队列", zap.String("task_id", task.ID.String()))
				default:
					logger.Warn("停止队列已满，无法重新放回任务", zap.String("task_id", task.ID.String()))
				}
			}
		}
	}
}

// ========== 说明注释 ==========

// 注意：pendingTasksQueue和stopTasksQueue是在dispatch.go中定义的全局变量
// 这里通过调用GetPendingTasksQueue和GetStopTasksQueue函数来获取这些队列
// 这些函数在dispatch.go中定义

// GetPendingTasks 获取当前待执行的任务
// 最多返回MaxTasksPerMessage个符合条件的任务
// 参数:
//   - ctx: 请求上下文
//
// 返回值:
//   - []*core.Task: 待执行任务列表
//   - error: 查询过程中的错误
func (w *WebsocketService) GetPendingTasks(ctx context.Context) ([]*core.Task, error) {
	// 获取当前时间
	now := time.Now()

	// 构建过滤器：Task.TimePlan <= now < Task.TimeoutAt 且状态是Pending
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "time_plan",
			Value:  now,
			Op:     filters.FILTER_LTE,
		},
		&filters.FilterOption{
			Column: "timeout_at",
			Value:  now,
			Op:     filters.FILTER_GT,
		},
		&filters.FilterOption{
			Column: "status",
			Value:  core.TaskStatusPending,
			Op:     filters.FILTER_EQ,
		},
	}

	// 从数据库获取待处理任务
	tasks, err := w.taskStore.List(ctx, 0, MaxTasksPerMessage, filterActions...)
	if err != nil {
		logger.Error("获取待处理任务失败", zap.Error(err))
		return nil, err
	}

	logger.Info("成功获取待处理任务列表", zap.Int("count", len(tasks)))
	return tasks, nil
}
