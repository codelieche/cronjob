package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/config"
	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/codelieche/cronjob/worker/pkg/utils/logger"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// websocketServiceImpl WebSocket服务实现
// 实现了core.WebsocketService接口
type websocketServiceImpl struct {
	conn           *websocket.Conn
	messageCache   string
	cacheMutex     sync.Mutex
	writeMutex     sync.Mutex
	done           chan struct{}
	config         *core.WebsocketConfig
	workerConfig   *config.Worker
	connected      bool
	connectedMutex sync.RWMutex
	taskService    core.TaskService
	reconnecting   bool
	reconnectMutex sync.Mutex
	apiserver      core.Apiserver
	pingStopChan   chan struct{} // 用于停止ping goroutine
	pingStopMutex  sync.Mutex    // 保护pingStopChan的并发访问
}

// NewWebsocketService 创建WebSocket服务实例
func NewWebsocketService() core.WebsocketService {
	// 从配置中获取WebSocket配置
	wsConfig := core.DefaultWebsocketConfig()
	wsConfig.ServerURL = config.Server.ApiUrl
	wsConfig.PingInterval = time.Duration(config.WebsocketPingInterval) * time.Second
	wsConfig.MessageSeparator = config.WebsocketMessageSeparator

	apiserver := NewApiserverService(config.Server.ApiUrl, config.Server.AuthToken)

	ws := &websocketServiceImpl{
		config:       wsConfig,
		workerConfig: config.WorkerInstance,
		done:         make(chan struct{}),
		apiserver:    apiserver,
		pingStopChan: make(chan struct{}),
	}

	// 创建Task服务
	ws.taskService = NewTaskService(ws, apiserver)

	return ws
}

// Connect 连接到apiserver的WebSocket
func (ws *websocketServiceImpl) Connect() error {
	// 构建WebSocket URL
	wsUrl := strings.Replace(ws.config.ServerURL, "http://", "ws://", 1)
	wsUrl = strings.Replace(wsUrl, "https://", "wss://", 1)
	wsUrl = wsUrl + "/ws/task/"

	logger.Info("正在连接WebSocket", zap.String("url", wsUrl))

	// 解析URL
	u, err := url.Parse(wsUrl)
	if err != nil {
		return fmt.Errorf("解析WebSocket URL失败: %v", err)
	}

	// 建立WebSocket连接
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("连接WebSocket失败: %v", err)
	}

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(ws.config.ReadTimeout))

	// 设置pong处理器，当收到pong消息时重置读取超时
	conn.SetPongHandler(func(string) error {
		// logger.Info("收到pong消息，重置读取超时")
		// 我们可以在发送了Ping消息后，收到Pong消息时重置读取超时，也可以在SendPing没报错之后重置，2个地方都ok
		// conn.SetReadDeadline(time.Now().Add(ws.config.ReadTimeout))
		return nil
	})
	conn.SetReadLimit(int64(ws.config.MaxMessageSize))

	ws.conn = conn

	ws.setConnected(true)
	logger.Info("WebSocket连接成功")

	// 连接成功后立即注册Worker
	if err := ws.registerWorker(); err != nil {
		logger.Error("注册Worker失败", zap.Error(err))
		// 注册失败不影响连接，继续运行
		return err
	}

	return nil
}

// registerWorker 注册Worker到服务端
func (ws *websocketServiceImpl) registerWorker() error {
	// 构建Worker数据
	workerData, err := json.Marshal(ws.workerConfig)
	if err != nil {
		return fmt.Errorf("序列化Worker数据失败: %v", err)
	}

	// 构建注册事件
	clientEvent := core.ClientEvent{
		Action:   core.ClientEventActionRegistWorker,
		WorkerID: ws.workerConfig.ID.String(),
		Data:     workerData,
	}

	// 序列化事件
	eventData, err := json.Marshal(clientEvent)
	if err != nil {
		return fmt.Errorf("序列化ClientEvent失败: %v", err)
	}

	// 添加消息分隔符
	message := ws.config.MessageSeparator + string(eventData) + ws.config.MessageSeparator

	// 发送注册消息
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()

	if ws.conn == nil {
		return fmt.Errorf("WebSocket连接未建立")
	}

	ws.conn.SetWriteDeadline(time.Now().Add(ws.config.WriteTimeout))
	return ws.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

// Close 关闭WebSocket连接
func (ws *websocketServiceImpl) Close() {
	ws.setConnected(false)

	// 停止ping goroutine
	ws.stopPingPump()

	// 安全地关闭连接
	ws.writeMutex.Lock()
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
	ws.writeMutex.Unlock()

	// 关闭done通道，通知所有goroutines退出
	select {
	case <-ws.done:
		// 已经关闭
	default:
		close(ws.done)
	}
}

// IsConnected 检查连接状态
func (ws *websocketServiceImpl) IsConnected() bool {
	ws.connectedMutex.RLock()
	defer ws.connectedMutex.RUnlock()
	return ws.connected
}

// setConnected 设置连接状态
func (ws *websocketServiceImpl) setConnected(connected bool) {
	ws.connectedMutex.Lock()
	defer ws.connectedMutex.Unlock()
	ws.connected = connected
}

// stopPingPump 停止ping goroutine
func (ws *websocketServiceImpl) stopPingPump() {
	ws.pingStopMutex.Lock()
	defer ws.pingStopMutex.Unlock()

	select {
	case <-ws.pingStopChan:
		// 已经关闭了
	default:
		close(ws.pingStopChan)
	}
}

// Start 启动WebSocket服务
func (ws *websocketServiceImpl) Start() error {
	// 连接到apiserver
	if err := ws.Connect(); err != nil {
		logger.Error("连接apiserver失败，将自动重试", zap.Error(err))
		// 不直接退出，让重连机制处理
		return err
	}

	// 连接成功后执行Setup
	if err := ws.Setup(); err != nil {
		logger.Error("Worker Setup失败", zap.Error(err))
		ws.Close()
		return fmt.Errorf("Worker Setup失败: %w", err)
	}

	// 启动消息读取goroutine
	go ws.readPump()

	// 启动ping goroutine
	go ws.pingPump()

	return nil
}

// Stop 停止WebSocket服务
func (ws *websocketServiceImpl) Stop() {
	// 在关闭前执行Teardown
	// if err := ws.Teardown(); err != nil {
	// 	logger.Error("Worker Teardown失败", zap.Error(err))
	// 	// Teardown失败不影响关闭流程
	// }

	ws.Close()
}

// readPump 读取WebSocket消息的goroutine
func (ws *websocketServiceImpl) readPump() {
	defer func() {
		// 安全地关闭连接
		ws.writeMutex.Lock()
		if ws.conn != nil {
			ws.conn.Close()
			ws.conn = nil
		}
		ws.writeMutex.Unlock()
		ws.setConnected(false)
		logger.Info("WebSocket读取goroutine退出")
	}()

	// 检查连接是否有效
	ws.writeMutex.Lock()
	conn := ws.conn
	ws.writeMutex.Unlock()

	if conn == nil {
		logger.Error("WebSocket连接为nil，无法启动读取goroutine")
		return
	}

	for {
		// logger.Debug("WebSocket readPump running")
		select {
		case <-ws.done:
			logger.Info("readPump收到关闭信号，退出")
			return
		default:
			// 再次检查连接是否有效
			ws.writeMutex.Lock()
			conn = ws.conn
			ws.writeMutex.Unlock()

			if conn == nil {
				logger.Info("WebSocket连接已断开，读取goroutine退出")
				return
			}
			// 设置读取超时，防止长时间阻塞
			conn.SetReadDeadline(time.Now().Add(ws.config.ReadTimeout))
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				logger.Error("WebSocket读取消息失败",
					zap.Error(err),
					zap.String("message_type", fmt.Sprintf("%d", messageType)),
					zap.String("error_type", fmt.Sprintf("%T", err)),
					zap.Bool("is_timeout", err.Error() == "i/o timeout"))

				// 判断是否为异常关闭
				isUnexpectedClose := websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway)
				// 特别处理1006错误码（意外EOF）
				if closeErr, ok := err.(*websocket.CloseError); ok && closeErr.Code == websocket.CloseAbnormalClosure {
					isUnexpectedClose = true
				}

				if isUnexpectedClose {
					logger.Error("WebSocket连接异常关闭", zap.Error(err))
					// 只有在异常关闭时才触发重连
					go ws.reconnect()
				} else {
					logger.Info("WebSocket连接正常关闭", zap.Error(err))
					go ws.reconnect()
				}
				return
			}

			// 处理收到的消息
			ws.HandleMessage(message)
		}
	}
}

// pingPump 定时发送ping消息的goroutine
func (ws *websocketServiceImpl) pingPump() {
	ticker := time.NewTicker(ws.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.done:
			return
		case <-ws.pingStopChan:
			logger.Debug("收到ping停止信号，停止ping goroutine")
			return
		case <-ticker.C:
			// 检查是否正在重连
			ws.reconnectMutex.Lock()
			reconnecting := ws.reconnecting
			ws.reconnectMutex.Unlock()

			if reconnecting {
				logger.Debug("正在重连中，跳过ping")
				continue
			}

			// logger.Info("准备发送ping消息")
			if err := ws.SendPing(); err != nil {
				logger.Error("发送ping失败", zap.Error(err))
				// 发送失败，触发重连
				go ws.reconnect()
				return
			} else {
				// 发送ping成功后，重置读取超时（使用写锁保护）
				ws.writeMutex.Lock()
				// 可以在这里重置ReadDeadline也可以在conn.SetPongHandler中处理
				if ws.conn != nil {
					if err = ws.conn.SetReadDeadline(time.Now().Add(ws.config.ReadTimeout)); err != nil {
						logger.Error("设置读取超时失败", zap.Error(err))
					}
				}
				ws.writeMutex.Unlock()
			}
			logger.Info("ping消息发送成功")
		}
	}
}

// SendPing 发送ping消息
func (ws *websocketServiceImpl) SendPing() error {
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()

	if ws.conn == nil {
		return fmt.Errorf("WebSocket连接未建立")
	}

	// 使用WebSocket标准的ping消息，而不是自定义JSON
	ws.conn.SetWriteDeadline(time.Now().Add(ws.config.WriteTimeout))
	// 注意发送的data是nil，因为标准的ping消息不需要携带数据
	return ws.conn.WriteMessage(websocket.PingMessage, nil)
}

// SendTaskUpdate 发送任务更新
func (ws *websocketServiceImpl) SendTaskUpdate(taskID string, data map[string]interface{}) error {
	ws.writeMutex.Lock()
	defer ws.writeMutex.Unlock()

	if ws.conn == nil {
		return fmt.Errorf("WebSocket连接未建立")
	}

	// 构建ClientEvent
	eventData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化任务数据失败: %v", err)
	}

	clientEvent := core.ClientEvent{
		Action:   core.ClientEventActionTaskUpdate,
		WorkerID: ws.workerConfig.ID.String(),
		TaskID:   taskID,
		Data:     eventData,
	}

	// 序列化ClientEvent
	eventBytes, err := json.Marshal(clientEvent)
	if err != nil {
		return fmt.Errorf("序列化ClientEvent失败: %v", err)
	}

	// 添加消息分隔符
	message := ws.config.MessageSeparator + string(eventBytes) + ws.config.MessageSeparator

	ws.conn.SetWriteDeadline(time.Now().Add(ws.config.WriteTimeout))
	// 现在需要对message分片发送
	if len(message) > 1024 {
		// 消息长度超过1024，需要分片发送
		for i := 0; i < len(message); i += 512 {
			end := i + 512
			if end > len(message) {
				end = len(message)
			}
			if err := ws.conn.WriteMessage(websocket.TextMessage, []byte(message[i:end])); err != nil {
				logger.Error("发送任务更新消息失败", zap.Error(err))
				return err
			}
		}
		return nil
	} else {
		// 消息长度小于等于1024，直接发送
		if err := ws.conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			logger.Error("发送任务更新消息失败", zap.Error(err))
			return err
		}
		return nil
	}
}

// HandleMessage 处理收到的消息
func (ws *websocketServiceImpl) HandleMessage(message []byte) {
	messageStr := string(message)
	logger.Debug("收到服务端消息", zap.String("message", messageStr), zap.Int("length", len(messageStr)))

	// 注意：标准的WebSocket pong消息会被WebSocket库自动处理，
	// 不需要在这里特殊处理，这里只处理业务消息

	// 合并缓存的不完整消息
	ws.cacheMutex.Lock()
	fullMessage := ws.messageCache + messageStr
	ws.cacheMutex.Unlock()

	separator := ws.config.MessageSeparator
	var completeEvents []string
	var remainingMessage string

	// 解析消息
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
	ws.cacheMutex.Lock()
	if remainingMessage != "" {
		ws.messageCache = remainingMessage
	} else {
		ws.messageCache = ""
	}
	ws.cacheMutex.Unlock()

	// 处理每个完整的事件
	logger.Debug("解析到完整事件", zap.Int("count", len(completeEvents)))
	for i, eventStr := range completeEvents {
		logger.Debug("处理事件", zap.Int("index", i), zap.String("event", eventStr))
		var event core.TaskEvent
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			logger.Error("解析TaskEvent失败", zap.Error(err), zap.String("message", eventStr))
			continue
		}

		// 处理TaskEvent
		ws.taskService.HandleTaskEvent(&event)
	}
}

// isCompleteJSON 检查字符串是否是完整的JSON对象
func isCompleteJSON(str string) bool {
	str = strings.TrimSpace(str)
	if len(str) < 2 || str[0] != '{' || str[len(str)-1] != '}' {
		return false
	}

	var js map[string]interface{}
	err := json.Unmarshal([]byte(str), &js)
	return err == nil
}

// reconnect 重连WebSocket
func (ws *websocketServiceImpl) reconnect() {
	// 检查是否已经在重连中
	ws.reconnectMutex.Lock()
	if ws.reconnecting {
		ws.reconnectMutex.Unlock()
		logger.Debug("已经在重连中，跳过重复重连")
		return
	}
	ws.reconnecting = true
	ws.reconnectMutex.Unlock()

	defer func() {
		ws.reconnectMutex.Lock()
		ws.reconnecting = false
		ws.reconnectMutex.Unlock()
	}()

	logger.Info("开始重连WebSocket", zap.Duration("delay", ws.config.ReconnectDelay))

	// 安全地关闭当前连接
	ws.writeMutex.Lock()
	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
	ws.writeMutex.Unlock()
	ws.setConnected(false)

	// 等待重连延迟
	time.Sleep(ws.config.ReconnectDelay)

	// 尝试重连
	for {
		select {
		case <-ws.done:
			return
		default:
			if err := ws.Connect(); err != nil {
				logger.Error("重连失败", zap.Error(err))
				time.Sleep(ws.config.ReconnectDelay)
				continue
			}
			logger.Info("重连成功")

			// 重连成功后执行Setup
			if err := ws.Setup(); err != nil {
				logger.Error("重连后Worker Setup失败", zap.Error(err))
				ws.Close()
				time.Sleep(ws.config.ReconnectDelay)
				continue
			}

			// 停止旧的ping goroutine
			ws.stopPingPump()

			// 重新创建pingStopChan
			ws.pingStopMutex.Lock()
			ws.pingStopChan = make(chan struct{})
			ws.pingStopMutex.Unlock()

			// 重新启动读取和ping goroutines
			go ws.readPump()
			go ws.pingPump()
			return
		}
	}
}

// executeShellCommand 执行shell命令
func (ws *websocketServiceImpl) executeShellCommand(command string) (int, string, error) {
	if command == "" || strings.TrimSpace(command) == "" || command == "null" {
		return 0, "", nil
	}

	logger.Info("执行shell命令", zap.String("command", command))

	// 使用sh -c执行命令
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	exitCode := 0

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return -1, string(output), fmt.Errorf("执行命令失败: %w", err)
		}
	}

	logger.Info("命令执行完成",
		zap.String("command", command),
		zap.Int("exit_code", exitCode),
		zap.String("output", string(output)))

	return exitCode, string(output), nil
}

// Setup 设置WebSocket服务
func (ws *websocketServiceImpl) Setup() error {
	logger.Info("开始执行Worker Setup")

	// 获取Worker的Metadata.Tasks列表
	tasks := ws.workerConfig.Metadata.Tasks
	if len(tasks) == 0 {
		logger.Info("Worker没有配置任务类型，跳过Setup")
		return nil
	}

	logger.Info("Worker支持的任务类型", zap.Strings("tasks", tasks))

	// 遍历每个任务类型，获取对应的Category
	for _, taskType := range tasks {
		logger.Info("处理任务类型", zap.String("task_type", taskType))

		// 获取Category对象
		category, err := ws.apiserver.GetCategory(taskType)
		if err != nil {
			logger.Warn("获取Category失败，跳过",
				zap.String("task_type", taskType),
				zap.Error(err))
			continue
		}

		logger.Info("获取到Category",
			zap.String("task_type", taskType),
			zap.String("category_name", category.Name))

		// 先执行Check命令
		if category.Check != "" {
			logger.Info("执行Category Check命令",
				zap.String("task_type", taskType),
				zap.String("check_command", category.Check))

			exitCode, output, err := ws.executeShellCommand(category.Check)
			if err != nil {
				logger.Error("执行Check命令失败",
					zap.String("task_type", taskType),
					zap.Error(err))
				return fmt.Errorf("执行Check命令失败: %w", err)
			}

			// 如果Check命令成功（退出码为0），则跳过Setup
			if exitCode == 0 {
				logger.Info("Check命令执行成功，跳过Setup",
					zap.String("task_type", taskType),
					zap.String("output", output))
				continue
			}

			logger.Info("Check命令执行失败，需要执行Setup",
				zap.String("task_type", taskType),
				zap.Int("exit_code", exitCode),
				zap.String("output", output))
		}

		// 执行Setup命令
		if category.Setup != "" {
			logger.Info("执行Category Setup命令",
				zap.String("task_type", taskType),
				zap.String("setup_command", category.Setup))

			exitCode, output, err := ws.executeShellCommand(category.Setup)
			if err != nil {
				logger.Error("执行Setup命令失败",
					zap.String("task_type", taskType),
					zap.Error(err))
				return fmt.Errorf("执行Setup命令失败: %w", err)
			}

			if exitCode != 0 {
				logger.Error("Setup命令执行失败",
					zap.String("task_type", taskType),
					zap.Int("exit_code", exitCode),
					zap.String("output", output))
				return fmt.Errorf("Setup命令执行失败，退出码: %d, 输出: %s", exitCode, output)
			}

			logger.Info("Setup命令执行成功",
				zap.String("task_type", taskType),
				zap.String("output", output))
		}

		// 再次执行Check命令验证Setup是否成功
		if category.Check != "" {
			logger.Info("验证Setup结果，再次执行Check命令",
				zap.String("task_type", taskType),
				zap.String("check_command", category.Check))

			exitCode, output, err := ws.executeShellCommand(category.Check)
			if err != nil {
				logger.Error("验证Setup时执行Check命令失败",
					zap.String("task_type", taskType),
					zap.Error(err))
				return fmt.Errorf("验证Setup时执行Check命令失败: %w", err)
			}

			if exitCode != 0 {
				logger.Error("Setup验证失败，Check命令仍然失败",
					zap.String("task_type", taskType),
					zap.Int("exit_code", exitCode),
					zap.String("output", output))
				return fmt.Errorf("Setup验证失败，Check命令仍然失败，退出码: %d, 输出: %s", exitCode, output)
			}

			logger.Info("Setup验证成功",
				zap.String("task_type", taskType),
				zap.String("output", output))
		}
	}

	logger.Info("Worker Setup完成")
	return nil
}

// Teardown 卸载WebSocket服务
func (ws *websocketServiceImpl) Teardown() error {
	logger.Info("开始执行Worker Teardown")

	// 获取Worker的Metadata.Tasks列表
	tasks := ws.workerConfig.Metadata.Tasks
	if len(tasks) == 0 {
		logger.Info("Worker没有配置任务类型，跳过Teardown")
		return nil
	}

	logger.Info("Worker支持的任务类型", zap.Strings("tasks", tasks))

	// 遍历每个任务类型，获取对应的Category并执行Teardown
	for _, taskType := range tasks {
		logger.Info("处理任务类型", zap.String("task_type", taskType))

		// 获取Category对象
		category, err := ws.apiserver.GetCategory(taskType)
		if err != nil {
			logger.Warn("获取Category失败，跳过",
				zap.String("task_type", taskType),
				zap.Error(err))
			continue
		}

		logger.Info("获取到Category",
			zap.String("task_type", taskType),
			zap.String("category_name", category.Name))

		// 执行Teardown命令
		if category.Teardown != "" {
			logger.Info("执行Category Teardown命令",
				zap.String("task_type", taskType),
				zap.String("teardown_command", category.Teardown))

			exitCode, output, err := ws.executeShellCommand(category.Teardown)
			if err != nil {
				logger.Error("执行Teardown命令失败",
					zap.String("task_type", taskType),
					zap.Error(err))
				// Teardown失败不影响其他Category的处理，继续执行
				continue
			}

			logger.Info("Teardown命令执行完成",
				zap.String("task_type", taskType),
				zap.Int("exit_code", exitCode),
				zap.String("output", output))
		} else {
			logger.Info("Category没有配置Teardown命令，跳过",
				zap.String("task_type", taskType))
		}
	}

	logger.Info("Worker Teardown完成")
	return nil
}
