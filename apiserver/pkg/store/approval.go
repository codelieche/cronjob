package store

import (
	"context"
	"errors"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// approvalStore 审批存储实现
type approvalStore struct {
	db *gorm.DB
}

// NewApprovalStore 创建ApprovalStore实例
func NewApprovalStore(db *gorm.DB) core.ApprovalStore {
	return &approvalStore{
		db: db,
	}
}

// Create 创建审批
func (s *approvalStore) Create(ctx context.Context, approval *core.Approval) (*core.Approval, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(approval).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return approval, nil
}

// Update 更新审批
func (s *approvalStore) Update(ctx context.Context, approval *core.Approval) (*core.Approval, error) {
	// 检查是否存在
	if _, err := s.FindByID(ctx, approval.ID); err != nil {
		return nil, err
	}

	// 更新
	if err := s.db.Save(approval).Error; err != nil {
		return nil, err
	}

	return approval, nil
}

// FindByID 根据ID查找
func (s *approvalStore) FindByID(ctx context.Context, id uuid.UUID) (*core.Approval, error) {
	var approval core.Approval
	if err := s.db.First(&approval, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// DeleteByID 删除（软删除）
func (s *approvalStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查是否存在
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	// 硬删除
	if err := s.db.Delete(&core.Approval{}, "id = ?", id).Error; err != nil {
		return err
	}

	return nil
}

// List 获取列表（带过滤和分页）
func (s *approvalStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.Approval, error) {
	var approvals []*core.Approval
	query := s.db.Model(&core.Approval{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	// 排序：最新创建的在前
	query = query.Order("created_at DESC")

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Find(&approvals).Error; err != nil {
		return nil, err
	}

	return approvals, nil
}

// Count 统计数量
func (s *approvalStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	query := s.db.Model(&core.Approval{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// FindByTaskID 根据Task ID查找
func (s *approvalStore) FindByTaskID(ctx context.Context, taskID uuid.UUID) (*core.Approval, error) {
	var approval core.Approval
	if err := s.db.Where("task_id = ?", taskID).First(&approval).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &approval, nil
}

// FindTimeoutApprovals 查找超时的审批
func (s *approvalStore) FindTimeoutApprovals(ctx context.Context, now time.Time) ([]*core.Approval, error) {
	var approvals []*core.Approval
	// 状态为pending且已超时
	if err := s.db.Where("status = ? AND timeout_at <= ?", "pending", now).Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}

// FindMyPending 查找我的待审批
func (s *approvalStore) FindMyPending(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*core.Approval, error) {
	var approvals []*core.Approval
	// 使用JSON_CONTAINS查询user_ids中包含userID的记录
	query := s.db.Where("status = ? AND JSON_CONTAINS(user_ids, ?)", "pending", `"`+userID.String()+`"`)
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}

// FindMyCreated 查找我发起的审批
func (s *approvalStore) FindMyCreated(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*core.Approval, error) {
	var approvals []*core.Approval
	query := s.db.Where("created_by = ?", userID)
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}
