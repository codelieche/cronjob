package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/worker/pkg/core"
)

// ApprovalRunner 审批 Runner
//
// 用于在Workflow中创建审批步骤，支持：
// - 人工审批（指定user_ids）
// - AI自动审批（指定ai_agent_ids）
// - 混合审批（同时指定人工和AI）
// - 超时处理
//
// 工作流程：
// 1. 解析任务参数
// 2. 调用APIServer创建Approval对象
// 3. 将审批ID写入Task.Output
// 4. 设置Task状态为Running
// 5. 快速返回（非阻塞）
// 6. 等待审批人或AI通过API操作审批
// 7. 审批完成后，通过API更新Task状态
type ApprovalRunner struct {
	BaseRunner // 🔥 嵌入基类

	config ApprovalConfig
}

// ApprovalConfig 审批配置
type ApprovalConfig struct {
	Title      string   `json:"title"`        // 审批标题（必填）
	Content    string   `json:"content"`      // 审批内容（支持Markdown）
	Context    string   `json:"context"`      // 审批上下文（JSON格式，包含相关数据）
	UserIDs    []string `json:"user_ids"`     // 审批人用户ID列表
	AIAgentIDs []string `json:"ai_agent_ids"` // AI审批实体ID列表
	RequireAll bool     `json:"require_all"`  // 是否需要所有人都审批（默认false，任意一人即可）
	Timeout    int      `json:"timeout"`      // 审批超时时间（秒，可选，默认使用Task.Timeout，若Task.Timeout也为空则为3600）
	Metadata   string   `json:"metadata"`     // 扩展元数据（JSON格式）
}

// NewApprovalRunner 创建新的 ApprovalRunner
func NewApprovalRunner() *ApprovalRunner {
	r := &ApprovalRunner{}
	r.InitBase() // 🔥 初始化基类
	return r
}

// ParseArgs 解析任务参数
func (r *ApprovalRunner) ParseArgs(task *core.Task) error {
	r.Task = task

	// 解析 args（JSON 字符串）
	if err := json.Unmarshal([]byte(task.Args), &r.config); err != nil {
		return fmt.Errorf("解析审批配置失败: %w", err)
	}

	// 验证必填字段
	if r.config.Title == "" {
		return fmt.Errorf("审批标题不能为空")
	}

	// 验证至少有一个审批人或AI
	if len(r.config.UserIDs) == 0 && len(r.config.AIAgentIDs) == 0 {
		return fmt.Errorf("至少需要指定一个审批人或AI实体")
	}

	// 设置默认超时时间
	// 优先级：config.Timeout > Task.Timeout > 默认3600秒
	if r.config.Timeout <= 0 {
		if task.Timeout > 0 {
			r.config.Timeout = task.Timeout // 🔥 复用Task.Timeout
		} else {
			r.config.Timeout = 3600 // 默认1小时
		}
	}

	// 设置默认Context和Metadata
	if r.config.Context == "" {
		r.config.Context = "{}"
	}
	if r.config.Metadata == "" {
		r.config.Metadata = "{}"
	}

	return nil
}

// Execute 执行审批任务
//
// 核心逻辑：
// 1. 调用APIServer创建Approval对象
// 2. 获取审批ID
// 3. 构造输出（包含审批ID）
// 4. 快速返回Success
//
// 注意：此方法不会阻塞等待审批结果！
// 审批完成后，需要通过Approval API更新Task状态，触发Workflow继续执行。
func (r *ApprovalRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	r.Cancel = cancel
	defer cancel()

	r.StartTime = time.Now()
	r.Status = core.StatusRunning

	// 构建初始结果
	r.Result = &core.Result{
		Status:    core.StatusRunning,
		StartTime: r.StartTime,
	}

	// 记录开始日志
	logChan <- fmt.Sprintf("🔔 开始执行审批任务: %s\n", r.config.Title)
	logChan <- fmt.Sprintf("👥 审批人数量: %d, 🤖 AI实体数量: %d\n", len(r.config.UserIDs), len(r.config.AIAgentIDs))
	if r.config.RequireAll {
		logChan <- "📋 审批模式: 需要所有人都通过\n"
	} else {
		logChan <- "📋 审批模式: 任意一人通过即可\n"
	}
	logChan <- fmt.Sprintf("⏰ 超时时间: %d秒\n", r.config.Timeout)

	// 1. 创建Approval对象
	approvalID, err := r.createApproval(logChan)
	if err != nil {
		logChan <- fmt.Sprintf("❌ 创建审批失败: %v\n", err)
		return r.buildErrorResult("创建审批失败", err), err
	}

	logChan <- fmt.Sprintf("✅ 审批已创建，ID: %s\n", approvalID)
	logChan <- "📝 审批对象已创建，等待审批人或AI处理...\n"
	logChan <- "💡 提示：审批完成后，请通过 Approval API 操作审批状态\n"

	// 2. 构造输出
	output := map[string]interface{}{
		"approval_id":     approvalID,
		"approval_title":  r.config.Title,
		"approval_status": "pending",
		"user_count":      len(r.config.UserIDs),
		"ai_agent_count":  len(r.config.AIAgentIDs),
		"require_all":     r.config.RequireAll,
		"timeout":         r.config.Timeout,
		"message":         "审批已创建，等待处理",
		"created_at":      time.Now().Format(time.RFC3339),
	}

	outputBytes, _ := json.Marshal(output)

	// 3. 设置执行结果
	endTime := time.Now()
	duration := endTime.Sub(r.StartTime).Milliseconds()

	// 🔥 关键：返回 StatusRunning，表示审批正在进行中
	// Worker 会保持 Task 状态为 running，等待审批完成后通过 API 更新
	r.Result = &core.Result{
		Status:     core.StatusRunning, // ⚠️ 返回 running 而非 success
		Output:     string(outputBytes),
		ExecuteLog: fmt.Sprintf("审批已创建（ID: %s），等待审批人处理", approvalID),
		StartTime:  r.StartTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   0,
	}

	logChan <- "✅ ApprovalRunner 执行完成\n"
	logChan <- fmt.Sprintf("📄 审批ID已写入Task.Output: %s\n", approvalID)
	logChan <- "⏳ Task 状态保持为 running，等待审批完成\n"

	return r.Result, nil
}

// createApproval 调用APIServer创建Approval对象
func (r *ApprovalRunner) createApproval(logChan chan<- string) (string, error) {
	// 检查apiserver是否注入
	if r.Apiserver == nil {
		return "", fmt.Errorf("apiserver未初始化，无法创建审批")
	}

	logChan <- "📡 调用APIServer创建审批...\n"

	// 构造请求数据
	approvalData := map[string]interface{}{
		"title":        r.config.Title,
		"content":      r.config.Content,
		"context":      r.config.Context,
		"user_ids":     r.config.UserIDs,
		"ai_agent_ids": r.config.AIAgentIDs,
		"require_all":  r.config.RequireAll,
		"timeout":      r.config.Timeout,
		"metadata":     r.config.Metadata,
		"task_id":      r.Task.ID.String(),
		// 注意：workflow_exec_id从Task.Metadata中获取（如果需要）
	}

	// 调用Apiserver接口创建审批
	approvalID, err := r.Apiserver.CreateApproval(approvalData)
	if err != nil {
		return "", fmt.Errorf("创建审批失败: %w", err)
	}

	return approvalID, nil
}

// Stop 停止任务
func (r *ApprovalRunner) Stop() error {
	if r.Cancel != nil {
		r.Cancel()
	}
	r.Status = core.StatusStopped
	return nil
}

// Kill 强制终止任务
func (r *ApprovalRunner) Kill() error {
	if r.Cancel != nil {
		r.Cancel()
	}
	r.Status = core.StatusStopped
	return nil
}

// Cleanup 清理资源
func (r *ApprovalRunner) Cleanup() error {
	// ApprovalRunner 没有需要清理的资源
	return nil
}

// buildErrorResult 构造错误结果
func (r *ApprovalRunner) buildErrorResult(message string, err error) *core.Result {
	endTime := time.Now()
	duration := endTime.Sub(r.StartTime).Milliseconds()

	return &core.Result{
		Status:     core.StatusFailed,
		Output:     "",
		ExecuteLog: message,
		Error:      err.Error(),
		StartTime:  r.StartTime,
		EndTime:    endTime,
		Duration:   duration,
		ExitCode:   1,
	}
}
