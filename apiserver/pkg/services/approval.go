package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ApprovalService 审批服务实现
type ApprovalService struct {
	store               core.ApprovalStore
	approvalRecordStore core.ApprovalRecordStore
	taskStore           core.TaskStore
	workflowExecStore   core.WorkflowExecuteStore
	workflowExecService core.WorkflowExecuteService // 🔥 用于触发 Workflow 流转（使用接口）
	usercenterService   core.UsercenterService      // 🔥 用于发送站内信通知（使用接口）
}

// NewApprovalService 创建ApprovalService实例
func NewApprovalService(
	store core.ApprovalStore,
	approvalRecordStore core.ApprovalRecordStore,
	taskStore core.TaskStore,
	workflowExecStore core.WorkflowExecuteStore,
	workflowExecService core.WorkflowExecuteService, // 🔥 新增参数（使用接口）
	usercenterService core.UsercenterService, // 🔥 新增参数（使用接口）
) *ApprovalService {
	return &ApprovalService{
		store:               store,
		approvalRecordStore: approvalRecordStore,
		taskStore:           taskStore,
		workflowExecStore:   workflowExecStore,
		workflowExecService: workflowExecService, // 🔥 保存引用
		usercenterService:   usercenterService,   // 🔥 保存引用
	}
}

// Create 创建审批
// ⭐ 自动填充team_id：如果为空，使用当前用户的team_id
// ⭐ 自动填充created_by：优先级：请求中的created_by > WorkflowExec执行者 > 当前用户
func (s *ApprovalService) Create(ctx context.Context, approval *core.Approval, currentUserID, currentUserTeamID uuid.UUID) (*core.Approval, error) {
	// 自动填充team_id
	if approval.TeamID == uuid.Nil {
		approval.TeamID = currentUserTeamID
		logger.Info("auto fill team_id for approval",
			zap.String("approval_title", approval.Title),
			zap.String("team_id", currentUserTeamID.String()))
	}

	// 自动填充created_by
	if approval.CreatedBy == nil || *approval.CreatedBy == uuid.Nil {
		createdBy := s.determineCreatedBy(ctx, approval, currentUserID)
		approval.CreatedBy = &createdBy
		logger.Info("auto fill created_by for approval",
			zap.String("approval_title", approval.Title),
			zap.String("created_by", createdBy.String()))
	}

	// 🔥 处理JSON字段：确保json.RawMessage不为nil或空
	if len(approval.Context) == 0 {
		approval.Context = json.RawMessage("{}")
	}
	if len(approval.UserIDs) == 0 {
		approval.UserIDs = json.RawMessage("[]")
	}
	if len(approval.AIAgentIDs) == 0 {
		approval.AIAgentIDs = json.RawMessage("[]")
	}
	if len(approval.Metadata) == 0 {
		approval.Metadata = json.RawMessage("{}")
	}
	// AIDecision 初始为空，不设置

	// 设置开始时间和超时时间
	now := time.Now()
	approval.StartedAt = &now
	timeoutAt := now.Add(time.Duration(approval.Timeout) * time.Second)
	approval.TimeoutAt = &timeoutAt

	// 设置默认状态
	if approval.Status == "" {
		approval.Status = "pending"
	}

	// 创建审批
	created, err := s.store.Create(ctx, approval)
	if err != nil {
		logger.Error("create approval error", zap.Error(err))
		return nil, err
	}

	// 🔥 发送站内信通知审批人
	if s.usercenterService != nil {
		if err := s.sendNotificationToApprovers(created); err != nil {
			// 发送通知失败不影响审批创建（只记录日志）
			logger.Error("发送审批通知失败", zap.Error(err),
				zap.String("approval_id", created.ID.String()))
		}
	} else {
		logger.Warn("usercenterService未注入，跳过发送审批通知",
			zap.String("approval_id", created.ID.String()))
	}

	return created, nil
}

// determineCreatedBy 确定created_by的值
// 优先级：
// 1. 如果请求中有created_by（非空），使用它
// 2. 如果有workflow_exec_id，查找其执行者（UserID）
// 3. 使用当前用户ID
func (s *ApprovalService) determineCreatedBy(ctx context.Context, approval *core.Approval, currentUserID uuid.UUID) uuid.UUID {
	// 1. 如果请求中已经有created_by，使用它
	if approval.CreatedBy != nil && *approval.CreatedBy != uuid.Nil {
		return *approval.CreatedBy
	}

	// 2. 如果有workflow_exec_id，查找其执行者
	if approval.WorkflowExecID != nil && *approval.WorkflowExecID != uuid.Nil {
		workflowExec, err := s.workflowExecStore.FindByID(ctx, *approval.WorkflowExecID)
		if err == nil && workflowExec.UserID != nil && *workflowExec.UserID != uuid.Nil {
			logger.Info("use workflow executor as created_by",
				zap.String("workflow_exec_id", approval.WorkflowExecID.String()),
				zap.String("executor", workflowExec.UserID.String()))
			return *workflowExec.UserID
		}
	}

	// 3. 使用当前用户ID
	return currentUserID
}

// Update 更新审批
func (s *ApprovalService) Update(ctx context.Context, approval *core.Approval) (*core.Approval, error) {
	updated, err := s.store.Update(ctx, approval)
	if err != nil {
		logger.Error("update approval error", zap.Error(err))
		return nil, err
	}

	return updated, nil
}

// FindByID 根据ID查找
func (s *ApprovalService) FindByID(ctx context.Context, id string) (*core.Approval, error) {
	approvalID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse approval id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	approval, err := s.store.FindByID(ctx, approvalID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find approval by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	return approval, nil
}

// DeleteByID 删除
func (s *ApprovalService) DeleteByID(ctx context.Context, id string) error {
	approvalID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse approval id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	if err := s.store.DeleteByID(ctx, approvalID); err != nil {
		logger.Error("delete approval error", zap.Error(err), zap.String("id", id))
		return err
	}

	return nil
}

// List 获取列表
func (s *ApprovalService) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.Approval, error) {
	approvals, err := s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list approvals error", zap.Error(err))
		return nil, err
	}

	return approvals, nil
}

// Count 统计数量
func (s *ApprovalService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count approvals error", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// HandleAction 处理审批操作（approve/reject/cancel）
func (s *ApprovalService) HandleAction(ctx context.Context, approvalID string, action string, comment string, userID uuid.UUID) error {
	// 1. 查找审批
	approval, err := s.FindByID(ctx, approvalID)
	if err != nil {
		return err
	}

	// 2. 检查状态
	if approval.Status != "pending" {
		return fmt.Errorf("审批状态不是pending，无法操作")
	}

	// 3. 检查权限（简化处理，后续可以增强）
	// TODO: 检查当前用户是否在审批人列表中

	// 4. 更新审批状态
	now := time.Now()
	switch action {
	case "approve":
		approval.Status = "approved"
		approval.ApprovedBy = userID.String()
		approval.ApprovedAt = &now
		approval.ApprovalComment = comment
	case "reject":
		approval.Status = "rejected"
		approval.ApprovedBy = userID.String()
		approval.ApprovedAt = &now
		approval.ApprovalComment = comment
	case "cancel":
		approval.Status = "cancelled"
		approval.ApprovalComment = comment
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	// 5. 保存审批
	if _, err := s.Update(ctx, approval); err != nil {
		return err
	}

	// 6. 记录操作历史
	record := &core.ApprovalRecord{
		ApprovalID: approval.ID,
		Action:     action,
		UserID:     &userID,
		Comment:    comment,
		Metadata:   json.RawMessage("{}"), // 🔥 传递空JSON对象，而不是nil
	}
	if _, err := s.approvalRecordStore.Create(ctx, record); err != nil {
		logger.Error("create approval record error", zap.Error(err))
		// 不影响主流程
	}

	// 7. 如果有关联的Task，更新Task状态
	if approval.TaskID != nil && *approval.TaskID != uuid.Nil {
		if err := s.updateTaskStatus(ctx, *approval.TaskID, approval.Status); err != nil {
			logger.Error("update task status error", zap.Error(err))
			// 不影响主流程，但记录错误
		}
	}

	return nil
}

// updateTaskStatus 更新Task状态（根据审批结果）
func (s *ApprovalService) updateTaskStatus(ctx context.Context, taskID uuid.UUID, approvalStatus string) error {
	task, err := s.taskStore.FindByID(ctx, taskID)
	if err != nil {
		return err
	}

	// 根据审批状态设置Task状态
	var taskStatus string
	switch approvalStatus {
	case "approved":
		taskStatus = "success"
	case "rejected":
		taskStatus = "failed"
	case "cancelled":
		taskStatus = "cancelled"
	case "timeout":
		taskStatus = "timeout"
	default:
		return fmt.Errorf("未知的审批状态: %s", approvalStatus)
	}

	task.Status = taskStatus
	now := time.Now()
	task.TimeEnd = &now // 🔥 设置结束时间
	task.UpdatedAt = time.Now()

	// 更新Task
	if _, err := s.taskStore.Update(ctx, task); err != nil {
		return err
	}

	// 🔥 触发 Workflow 流转（关键！）
	if s.workflowExecService != nil && task.WorkflowExecID != nil {
		logger.Info("触发 Workflow 流转",
			zap.String("task_id", taskID.String()),
			zap.String("task_status", taskStatus),
			zap.String("approval_status", approvalStatus))

		if err := s.workflowExecService.HandleTaskComplete(ctx, taskID); err != nil {
			logger.Error("触发 Workflow 流转失败", zap.Error(err))
			return err
		}
	}

	return nil
}

// FindMyPending 查找我的待审批
func (s *ApprovalService) FindMyPending(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*core.Approval, error) {
	approvals, err := s.store.FindMyPending(ctx, userID, offset, limit)
	if err != nil {
		logger.Error("find my pending approvals error", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, err
	}
	return approvals, nil
}

// FindMyCreated 查找我发起的审批
func (s *ApprovalService) FindMyCreated(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*core.Approval, error) {
	approvals, err := s.store.FindMyCreated(ctx, userID, offset, limit)
	if err != nil {
		logger.Error("find my created approvals error", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, err
	}
	return approvals, nil
}

// HandleTimeout 处理超时的审批
func (s *ApprovalService) HandleTimeout(ctx context.Context) error {
	now := time.Now()
	approvals, err := s.store.FindTimeoutApprovals(ctx, now)
	if err != nil {
		logger.Error("find timeout approvals error", zap.Error(err))
		return err
	}

	logger.Info("found timeout approvals", zap.Int("count", len(approvals)))

	for _, approval := range approvals {
		approval.Status = "timeout"
		if _, err := s.Update(ctx, approval); err != nil {
			logger.Error("update timeout approval error", zap.Error(err), zap.String("approval_id", approval.ID.String()))
			continue
		}

		// 如果有关联的Task，更新Task状态
		if approval.TaskID != nil && *approval.TaskID != uuid.Nil {
			if err := s.updateTaskStatus(ctx, *approval.TaskID, "timeout"); err != nil {
				logger.Error("update task status on timeout error", zap.Error(err))
			}
		}

		logger.Info("approval timeout handled", zap.String("approval_id", approval.ID.String()))
	}

	return nil
}

// sendNotificationToApprovers 发送通知给审批人
//
// 说明:
//   - 解析审批人列表，批量发送站内信
//   - 发送失败不影响主流程（只记录日志）
//   - 消息包含审批标题、内容和关联信息
func (s *ApprovalService) sendNotificationToApprovers(approval *core.Approval) error {
	// 1. 解析审批人列表
	userIDs, err := approval.GetUserIDs()
	if err != nil {
		return fmt.Errorf("解析审批人列表失败: %w", err)
	}

	if len(userIDs) == 0 {
		logger.Warn("审批没有配置审批人，跳过发送通知",
			zap.String("approval_id", approval.ID.String()),
			zap.String("title", approval.Title))
		return nil
	}

	// 2. 构建消息请求列表
	var messageReqs []*core.MessageCreateRequest
	for _, userIDStr := range userIDs {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			logger.Warn("解析审批人ID失败",
				zap.String("user_id", userIDStr),
				zap.Error(err))
			continue
		}

		// 构建消息内容
		messageReq := &core.MessageCreateRequest{
			ReceiverID:  userID,
			TeamID:      &approval.TeamID,
			Platform:    "apiserver",
			Category:    "info",
			Title:       fmt.Sprintf("【待审批】%s", approval.Title),
			Content:     approval.Content,
			RelatedID:   &approval.ID,
			RelatedType: "approval",
			SenderID:    approval.CreatedBy,
		}

		messageReqs = append(messageReqs, messageReq)
	}

	// 3. 批量发送消息
	if len(messageReqs) == 0 {
		logger.Warn("没有有效的审批人ID，跳过发送通知",
			zap.String("approval_id", approval.ID.String()))
		return nil
	}

	if err := s.usercenterService.BatchCreateMessages(messageReqs); err != nil {
		return fmt.Errorf("批量发送消息失败: %w", err)
	}

	logger.Info("审批通知发送成功",
		zap.String("approval_id", approval.ID.String()),
		zap.String("title", approval.Title),
		zap.Int("approver_count", len(messageReqs)))

	return nil
}
