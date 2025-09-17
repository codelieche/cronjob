package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocket升级器配置
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许所有CORS请求，实际环境中应该根据需求进行限制
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebsocketController 处理WebSocket连接及消息通信的控制器
// 负责处理客户端连接、消息收发、事件处理等WebSocket相关功能
type WebsocketController struct {
	controllers.BaseController
	service      core.WebsocketService // WebSocket服务接口
	messageCache map[string]string     // 消息缓存，用于存储每个客户端的不完整消息
	cacheMutex   sync.Mutex            // 用于保护messageCache的互斥锁
}

// NewWebsocketController 创建WebsocketController实例
// 入参：
//   - service: WebSocket服务接口实现
//
// 返回值：
//   - *WebsocketController: WebSocket控制器实例
func NewWebsocketController(service core.WebsocketService) *WebsocketController {
	return &WebsocketController{
		service:      service,
		messageCache: make(map[string]string),
	}
}

// HandleConnect 处理WebSocket连接请求
// 功能：
//   - 将HTTP连接升级为WebSocket连接
//   - 创建客户端实例并添加到客户端管理器
//   - 向客户端发送待执行任务
//   - 启动消息读取循环
func (wc *WebsocketController) HandleConnect(c *gin.Context) {
	// 升级HTTP连接到WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("升级WebSocket连接失败", zap.Error(err))
		wc.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 创建客户端ID并实例化客户端
	clientID := uuid.New().String()
	client := services.NewClient(clientID, conn)

	// 获取客户端管理器并添加客户端
	clientManager := wc.service.GetClientManager()
	clientManager.Add(client)

	// 异步发送待执行任务和启动消息读取循环
	go wc.sendPendingTasksToClient(c.Request.Context(), client)
	go wc.readPump(clientID, conn, clientManager)
}

// readPump 读取客户端消息的goroutine
// 功能：
//   - 持续读取客户端发送的消息
//   - 处理连接断开和异常情况
//   - 设置读取超时和Pong消息处理
//   - 调用handleClientMessage处理收到的消息
func (wc *WebsocketController) readPump(clientID string, conn *websocket.Conn, clientManager core.WebsocketClientManager) {
	// 延迟清理函数，确保连接断开时资源被正确释放
	defer func() {
		clientManager.Remove(clientID) // 从管理器中移除客户端
		conn.Close()                   // 关闭WebSocket连接
		// 清理客户端的消息缓存
		wc.cacheMutex.Lock()
		delete(wc.messageCache, clientID)
		wc.cacheMutex.Unlock()
	}()

	// 配置WebSocket连接参数
	conn.SetReadLimit(1024) // 设置读取消息的大小限制
	// 设置pong消息处理，延长读取超时
	conn.SetPongHandler(func(string) error {
		logger.Info("收到pong消息，重置读取超时", zap.String("client_id", clientID))
		// conn.SetReadDeadline(time.Now().Add(90 * time.Second)) // 增加超时时间到90秒
		return nil
	})

	// 我们可以启动一个携程来，不断的ping客户端

	// 持续读取客户端消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// 处理连接断开的情况：worker会再次自动发起重连的
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket连接异常关闭", zap.Error(err), zap.String("client_id", clientID))
			} else {
				logger.Info("WebSocket连接正常关闭", zap.String("client_id", clientID))
			}
			break
		}

		// 处理收到的客户端消息
		wc.handleClientMessage(clientID, conn, message)
	}
}

// handleClientMessage 处理客户端发送的消息
// 功能：
//   - 合并缓存的不完整消息
//   - 根据分隔符或JSON完整性提取完整的事件消息
//   - 解析事件并分发给对应的处理函数
func (wc *WebsocketController) handleClientMessage(clientID string, conn *websocket.Conn, message []byte) {
	messageStr := string(message)
	logger.Debug("收到客户端消息", zap.String("client_id", clientID), zap.String("message", messageStr))

	// 从缓存中获取之前可能不完整的消息并合并
	wc.cacheMutex.Lock()
	cachedMessage := wc.messageCache[clientID]
	wc.cacheMutex.Unlock()

	fullMessage := cachedMessage + messageStr
	separator := config.WebsocketMessageSeparator

	var completeEvents []string // 完整的事件消息列表
	var remainingMessage string // 不完整的消息部分

	// 消息解析处理逻辑
	if separator != "" {
		// 使用分隔符解析消息
		sepPositions := []int{}
		currentPos := 0
		sepLen := len(separator)

		// 查找所有分隔符位置
		for {
			pos := strings.Index(fullMessage[currentPos:], separator)
			if pos == -1 {
				break
			}
			sepPositions = append(sepPositions, currentPos+pos)
			currentPos += pos + sepLen
		}

		// 提取完整的事件消息
		for i := 0; i < len(sepPositions)-1; i++ {
			start := sepPositions[i] + sepLen
			end := sepPositions[i+1]
			content := strings.TrimSpace(fullMessage[start:end])
			if content != "" {
				completeEvents = append(completeEvents, content)
			}
		}

		// 处理剩余的不完整消息
		if len(sepPositions) > 0 {
			lastSepPos := sepPositions[len(sepPositions)-1]
			if lastSepPos < len(fullMessage)-sepLen {
				remainingMessage = fullMessage[lastSepPos+sepLen:]
			} else {
				remainingMessage = ""
			}
		} else {
			// 没有找到分隔符，整个消息都是不完整的
			remainingMessage = fullMessage
		}
	} else {
		// 没有设置分隔符，尝试直接检查整个消息是否是完整的JSON
		if isCompleteJSON(fullMessage) {
			completeEvents = append(completeEvents, fullMessage)
			remainingMessage = ""
		} else {
			remainingMessage = fullMessage
		}
	}

	// 更新消息缓存
	wc.cacheMutex.Lock()
	if remainingMessage != "" {
		wc.messageCache[clientID] = remainingMessage
	} else {
		delete(wc.messageCache, clientID) // 清理缓存
	}
	wc.cacheMutex.Unlock()

	// 处理每个完整的事件
	for _, eventStr := range completeEvents {
		var event core.ClientEvent
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			logger.Error("解析ClientEvent失败", zap.Error(err), zap.String("message", eventStr))
			continue
		}

		// 根据事件类型分发到对应的处理函数
		switch event.Action {
		case core.ClientEventActionRegistWorker:
			wc.handleRegistWorkerEvent(context.Background(), clientID, &event)
		case core.ClientEventActionTaskUpdate:
			wc.handleTaskUpdateEvent(context.Background(), &event)
		default:
			logger.Warn("未知的客户端事件类型", zap.String("action", event.Action))
		}
	}
}

// isCompleteJSON 检查字符串是否是完整的JSON对象
// 简单的验证方法：
//  1. 检查是否以{开头，以}结尾
//  2. 尝试解析JSON以验证语法正确性
func isCompleteJSON(str string) bool {
	// 快速检查：JSON对象应该以{开头，以}结尾
	str = strings.TrimSpace(str)
	if len(str) < 2 || str[0] != '{' || str[len(str)-1] != '}' {
		return false
	}

	// 尝试解析JSON，看是否有语法错误
	var js map[string]interface{}
	err := json.Unmarshal([]byte(str), &js)
	return err == nil
}

// handleRegistWorkerEvent 处理Worker注册事件
// 功能：
//   - 验证WorkerID有效性
//   - 查找或创建Worker记录
//   - 更新Worker信息（名称、描述、元数据等）
//   - 设置Worker为活跃状态
func (wc *WebsocketController) handleRegistWorkerEvent(ctx context.Context, clientID string, event *core.ClientEvent) {
	// 验证WorkerID是否存在
	if event.WorkerID == "" {
		logger.Warn("注册Worker事件中WorkerID为空")
		return
	}

	// 解析WorkerID
	workerUUID, err := uuid.Parse(event.WorkerID)
	if err != nil {
		logger.Error("解析WorkerID失败", zap.Error(err), zap.String("worker_id", event.WorkerID))
		return
	}

	// 查找是否已存在该Worker
	worker, err := wc.service.GetWorkerByID(ctx, event.WorkerID)
	isUpdate := false
	if err != nil && err != core.ErrNotFound {
		logger.Error("查找Worker失败", zap.Error(err), zap.String("worker_id", event.WorkerID))
		return
	}

	// 准备Worker对象（创建新的或使用已存在的）
	if worker == nil {
		worker = &core.Worker{
			ID: workerUUID,
		}
	} else {
		isUpdate = true
	}

	// 更新Worker的通用字段
	isActive := true
	worker.IsActive = &isActive
	now := time.Now()
	worker.LastActive = &now

	// 如果有Data字段，解析并更新Worker详细信息
	if event.Data != nil {
		var workerData core.Worker
		if err := json.Unmarshal(event.Data, &workerData); err != nil {
			logger.Error("解析Worker数据失败", zap.Error(err))
		} else {
			// 校验ID是否匹配
			if workerData.ID != uuid.Nil && workerData.ID != workerUUID {
				logger.Error("WorkerID不匹配",
					zap.String("event_worker_id", event.WorkerID),
					zap.String("data_id", workerData.ID.String()))
				return
			}

			// 更新Worker字段信息
			if workerData.Name != "" {
				worker.Name = workerData.Name
			}
			worker.Description = workerData.Description
			if workerData.Metadata != nil {
				worker.Metadata = workerData.Metadata
			}
		}
	}

	// 保存Worker信息（创建或更新）
	if worker.ID == uuid.Nil || !isUpdate {
		_, err = wc.service.CreateWorker(ctx, worker)
	} else {
		_, err = wc.service.UpdateWorker(ctx, worker)
	}

	// 记录操作结果
	if err != nil {
		logger.Error("保存Worker失败", zap.Error(err), zap.String("worker_id", worker.ID.String()))
	} else {
		logger.Info("注册Worker成功", zap.String("worker_id", worker.ID.String()))
		// 将Worker信息保存到客户端管理器的workers映射中
		clientManager := wc.service.GetClientManager().(*services.ClientManagerImpl)
		// 注册worker信息
		clientManager.RegistWorker(clientID, worker)
	}

}

// handleTaskUpdateEvent 处理任务更新事件
// 功能：
//   - 验证TaskID有效性
//   - 检查任务是否可更新（未完成）
//   - 解析并更新任务字段（状态、输出、Worker信息等）
//   - 根据任务状态自动设置相关时间字段
func (wc *WebsocketController) handleTaskUpdateEvent(ctx context.Context, event *core.ClientEvent) {
	// 验证TaskID是否存在
	if event.TaskID == "" {
		logger.Warn("任务更新事件中TaskID为空")
		return
	}

	// 查找任务信息
	task, err := wc.service.GetTaskByID(ctx, event.TaskID)
	if err != nil {
		if err == core.ErrNotFound {
			logger.Warn("任务不存在", zap.String("task_id", event.TaskID))
		} else {
			logger.Error("查找任务失败", zap.Error(err), zap.String("task_id", event.TaskID))
		}
		return
	}

	// 检查任务是否已经完成（如果已设置结束时间则表示已完成）
	if task.TimeEnd != nil {
		logger.Warn("任务已经完成，不允许更新", zap.String("task_id", event.TaskID))
		return
	}

	// 准备要更新的字段
	updates := make(map[string]interface{})

	// 如果有Data字段，解析并提取要更新的任务信息
	if event.Data != nil {
		var taskData map[string]interface{}
		if err := json.Unmarshal(event.Data, &taskData); err != nil {
			logger.Error("解析Task数据失败", zap.Error(err))
		} else {
			// 处理状态字段更新
			if status, ok := taskData["status"].(string); ok {
				// 验证状态是否有效
				validStatus := map[string]bool{
					core.TaskStatusPending:  true,
					core.TaskStatusRunning:  true,
					core.TaskStatusSuccess:  true,
					core.TaskStatusFailed:   true,
					core.TaskStatusError:    true,
					core.TaskStatusTimeout:  true,
					core.TaskStatusCanceled: true,
					core.TaskStatusRetrying: true,
				}

				if _, isValid := validStatus[status]; !isValid {
					logger.Error("无效的任务状态", zap.String("status", status))
				} else {
					updates["status"] = status

					// 根据状态自动设置相关时间字段
					now := time.Now()
					if status == core.TaskStatusRunning {
						// 任务开始运行，设置开始时间
						updates["time_start"] = now
					} else if status != core.TaskStatusPending {
						// 任务完成或失败，设置结束时间
						updates["time_end"] = now
					}
				}
			}

			// 处理其他允许的字段更新
			if next, ok := taskData["next"].(string); ok && next != "" {
				nextUUID, err := uuid.Parse(next)
				if err == nil {
					updates["next"] = nextUUID
				} else {
					logger.Error("解析Next字段失败", zap.Error(err))
				}
			}

			if output, ok := taskData["output"].(string); ok {
				updates["output"] = output
			}
			if workerID, ok := taskData["worker_id"].(string); ok && workerID != "" {
				workerUUID, err := uuid.Parse(workerID)
				if err == nil {
					updates["worker_id"] = workerUUID
				} else {
					logger.Error("解析WorkerID字段失败", zap.Error(err))
				}
			}

			if workerName, ok := taskData["worker_name"].(string); ok {
				updates["worker_name"] = workerName
			}
		}
	}

	// 应用更新到任务
	if len(updates) > 0 {
		if err := wc.service.UpdateTaskFields(ctx, event.TaskID, updates); err != nil {
			logger.Error("更新任务字段失败", zap.Error(err), zap.String("task_id", event.TaskID))
		} else {
			logger.Info("更新任务成功", zap.String("task_id", event.TaskID))
		}
	} else {
		logger.Warn("没有需要更新的任务字段", zap.String("task_id", event.TaskID))
	}
}

// sendPendingTasksToClient 发送待执行任务给客户端
// 功能：
//   - 获取所有处于Pending状态的任务
//   - 分批发送任务给客户端（每批最多MaxTasksPerMessage条）
//   - 发送过程中添加短暂延迟避免发送过快
func (wc *WebsocketController) sendPendingTasksToClient(ctx context.Context, client core.WebsocketClient) {
	// 从数据库获取待执行任务
	pendingTasks, err := wc.service.GetPendingTasks(context.Background())
	if err != nil {
		logger.Error("获取待执行任务失败", zap.Error(err))
		return
	}

	// 分批发送任务给客户端，避免单次发送过多
	var currentTasks []*core.Task
	for i, task := range pendingTasks {
		currentTasks = append(currentTasks, task)

		// 如果达到最大发送数量或者是最后一条任务，发送当前批次
		if len(currentTasks) >= services.MaxTasksPerMessage || i == len(pendingTasks)-1 {
			event := &core.TaskEvent{
				Action: string(core.TaskActionRun),
				Tasks:  currentTasks,
			}

			// 发送消息给客户端
			if err := client.Send(event); err != nil {
				logger.Error("发送待执行任务失败", zap.Error(err), zap.String("client_id", client.ID()))
				break
			}

			// 重置当前任务列表，准备下一批
			currentTasks = []*core.Task{}

			// 短暂休眠，避免发送过快导致网络拥塞
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 记录发送结果
	logger.Info("已发送待执行任务给客户端",
		zap.String("client_id", client.ID()),
		zap.Int("total_tasks", len(pendingTasks)))
}
