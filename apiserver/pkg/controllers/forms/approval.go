package forms

import "github.com/google/uuid"

// ApprovalForm 审批创建表单
type ApprovalForm struct {
	TaskID         *uuid.UUID `json:"task_id"`                  // 关联Task ID（可选）
	WorkflowExecID *uuid.UUID `json:"workflow_exec_id"`         // 所属Workflow执行ID（可选）
	Title          string     `json:"title" binding:"required"` // 审批标题
	Content        string     `json:"content"`                  // 审批说明（Markdown格式）
	Context        string     `json:"context"`                  // 审批上下文数据（JSON字符串）
	UserIDs        []string   `json:"user_ids"`                 // 审批人ID列表
	AIAgentIDs     []string   `json:"ai_agent_ids"`             // AI Agent ID列表
	RequireAll     *bool      `json:"require_all"`              // 是否需要所有人审批（会签模式）
	Timeout        int        `json:"timeout"`                  // 审批超时时间（秒），默认3600
	Metadata       string     `json:"metadata"`                 // 扩展元数据（JSON字符串）
	TeamID         uuid.UUID  `json:"team_id"`                  // 团队ID（可选，不传则使用当前用户的team_id）
	CreatedBy      *uuid.UUID `json:"created_by"`               // 创建人ID（可选，不传则自动填充）
}

// ApprovalUpdateForm 审批更新表单
type ApprovalUpdateForm struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Context    string   `json:"context"`
	UserIDs    []string `json:"user_ids"`
	AIAgentIDs []string `json:"ai_agent_ids"`
	RequireAll *bool    `json:"require_all"`
	Timeout    int      `json:"timeout"`
	Metadata   string   `json:"metadata"`
}

// ApprovalActionForm 审批操作表单
type ApprovalActionForm struct {
	Action  string `json:"action" binding:"required,oneof=approve reject cancel"` // 操作：approve/reject/cancel
	Comment string `json:"comment"`                                               // 审批意见
}
