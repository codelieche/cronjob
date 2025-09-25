package core

// TaskUpdateCallback 任务更新回调接口
//
// 用于解决WebSocketService和TaskService之间的循环依赖问题
// TaskService通过此接口向WebSocketService发送任务更新，而不直接依赖WebSocketService
type TaskUpdateCallback interface {
	// SendTaskUpdate 发送任务状态更新
	//
	// 参数:
	//   - taskID: 任务ID
	//   - data: 更新数据，包含状态、时间、输出等信息
	//
	// 返回值:
	//   - error: 发送过程中的错误
	SendTaskUpdate(taskID string, data map[string]interface{}) error
}

// TaskEventHandler 任务事件处理接口
//
// 用于处理从WebSocket接收到的任务事件
// WebSocketService通过此接口将任务事件传递给TaskService处理
type TaskEventHandler interface {
	// HandleTaskEvent 处理任务事件
	//
	// 参数:
	//   - event: 任务事件对象，包含事件类型和任务列表
	HandleTaskEvent(event *TaskEvent)
}
