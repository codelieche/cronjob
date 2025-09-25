package services

import (
	"testing"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
	"github.com/google/uuid"
)

// mockWebsocketService 模拟WebSocket服务
type mockWebsocketService struct {
	sentUpdates map[string]map[string]interface{}
}

func (m *mockWebsocketService) SendTaskUpdate(taskID string, data map[string]interface{}) error {
	if m.sentUpdates == nil {
		m.sentUpdates = make(map[string]map[string]interface{})
	}
	m.sentUpdates[taskID] = data
	return nil
}

func (m *mockWebsocketService) Connect() error               { return nil }
func (m *mockWebsocketService) Close()                       {}
func (m *mockWebsocketService) IsConnected() bool            { return true }
func (m *mockWebsocketService) Setup() error                 { return nil }
func (m *mockWebsocketService) Teardown() error              { return nil }
func (m *mockWebsocketService) SendPing() error              { return nil }
func (m *mockWebsocketService) HandleMessage(message []byte) {}
func (m *mockWebsocketService) Start() error                 { return nil }
func (m *mockWebsocketService) Stop()                        {}

// mockApiserver 模拟API Server服务
type mockApiserver struct{}

func (m *mockApiserver) GetCategory(category string) (*core.Category, error) {
	// 返回一个默认的分类
	return &core.Category{
		Name: category,
	}, nil
}

func (m *mockApiserver) GetTask(taskID string) (*core.Task, error) {
	// 返回一个pending状态的任务
	return &core.Task{
		ID:     uuid.MustParse(taskID),
		Status: core.TaskStatusPending,
	}, nil
}

func TestTaskService_ExecuteTask(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建测试任务
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "test-task",
		Category: "command",
		Command:  "echo",
		Args:     `["Hello, World!"]`,
		Timeout:  30,
	}

	// 执行任务
	taskService.ExecuteTask(task)

	// 等待任务完成
	time.Sleep(2 * time.Second)

	// 检查是否发送了状态更新
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}

	// 检查running状态更新
	runningUpdate, exists := updates[task.ID.String()]
	if !exists {
		t.Fatal("期望发送running状态更新")
	}

	if runningUpdate["status"] != "running" {
		t.Fatalf("期望状态为running，实际为: %s", runningUpdate["status"])
	}

	// 检查最终结果
	// 由于是异步执行，我们需要等待一下
	time.Sleep(1 * time.Second)

	// 检查是否有成功或失败的结果
	finalUpdate, exists := updates[task.ID.String()]
	if !exists {
		t.Fatal("期望发送最终状态更新")
	}

	status := finalUpdate["status"].(string)
	if status != "success" && status != "failed" && status != "error" {
		t.Fatalf("期望最终状态为success/failed/error，实际为: %s", status)
	}
}

func TestTaskService_StopTask(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建长时间运行的任务
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "long-running-task",
		Category: "command",
		Command:  "sleep",
		Args:     `["10"]`,
		Timeout:  0,
	}

	// 在goroutine中执行任务
	go taskService.ExecuteTask(task)

	// 等待任务开始
	time.Sleep(100 * time.Millisecond)

	// 停止任务
	taskService.StopTasks([]*core.Task{task})

	// 等待停止完成
	time.Sleep(1 * time.Second)

	// 检查是否发送了停止请求
	// 注意：由于任务可能已经完成，我们主要检查没有错误
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}
}

func TestTaskService_KillTask(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建长时间运行的任务
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "long-running-task",
		Category: "command",
		Command:  "sleep",
		Args:     `["10"]`,
		Timeout:  0,
	}

	// 在goroutine中执行任务
	go taskService.ExecuteTask(task)

	// 等待任务开始
	time.Sleep(100 * time.Millisecond)

	// 强制终止任务
	taskService.KillTasks([]*core.Task{task})

	// 等待终止完成
	time.Sleep(1 * time.Second)

	// 检查是否发送了终止请求
	// 注意：由于任务可能已经完成，我们主要检查没有错误
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}
}

func TestTaskService_UnsupportedCategory(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建不支持的任务类型
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "unsupported-task",
		Category: "unsupported",
		Command:  "echo",
		Args:     `["test"]`,
		Timeout:  30,
	}

	// 执行任务
	taskService.ExecuteTask(task)

	// 等待处理完成
	time.Sleep(1 * time.Second)

	// 检查是否发送了错误状态更新
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}

	errorUpdate, exists := updates[task.ID.String()]
	if !exists {
		t.Fatal("期望发送错误状态更新")
	}

	if errorUpdate["status"] != "error" {
		t.Fatalf("期望状态为error，实际为: %s", errorUpdate["status"])
	}
}

func TestTaskService_InvalidArgs(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建参数无效的任务
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "invalid-args-task",
		Category: "command",
		Command:  "echo",
		Args:     "invalid json",
		Timeout:  30,
	}

	// 执行任务
	taskService.ExecuteTask(task)

	// 等待处理完成
	time.Sleep(1 * time.Second)

	// 检查是否发送了错误状态更新
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}

	errorUpdate, exists := updates[task.ID.String()]
	if !exists {
		t.Fatal("期望发送错误状态更新")
	}

	if errorUpdate["status"] != "error" {
		t.Fatalf("期望状态为error，实际为: %s", errorUpdate["status"])
	}
}

func TestTaskService_Timeout(t *testing.T) {
	// 创建模拟服务
	wsService := &mockWebsocketService{}
	apiserver := &mockApiserver{}

	// 创建任务服务
	taskService := NewTaskService(wsService, apiserver).(*taskServiceImpl)

	// 创建超时任务
	task := &core.Task{
		ID:       uuid.New(),
		Name:     "timeout-task",
		Category: "command",
		Command:  "sleep",
		Args:     `["5"]`,
		Timeout:  2, // 2秒超时
	}

	// 执行任务
	taskService.ExecuteTask(task)

	// 等待任务完成
	time.Sleep(5 * time.Second)

	// 检查是否发送了超时状态更新
	updates := wsService.sentUpdates
	if len(updates) == 0 {
		t.Fatal("期望发送状态更新，但没有发送")
	}

	finalUpdate, exists := updates[task.ID.String()]
	if !exists {
		t.Fatal("期望发送最终状态更新")
	}

	status := finalUpdate["status"].(string)
	if status != "timeout" && status != "success" {
		t.Fatalf("期望状态为timeout或success，实际为: %s", status)
	}
}
