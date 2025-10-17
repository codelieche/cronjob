package store

import (
	"context"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// approvalRecordStore 审批记录存储实现
type approvalRecordStore struct {
	db *gorm.DB
}

// NewApprovalRecordStore 创建ApprovalRecordStore实例
func NewApprovalRecordStore(db *gorm.DB) core.ApprovalRecordStore {
	return &approvalRecordStore{
		db: db,
	}
}

// Create 创建审批记录
func (s *approvalRecordStore) Create(ctx context.Context, record *core.ApprovalRecord) (*core.ApprovalRecord, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(record).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return record, nil
}

// FindByApprovalID 根据审批ID查找记录列表
func (s *approvalRecordStore) FindByApprovalID(ctx context.Context, approvalID uuid.UUID) ([]*core.ApprovalRecord, error) {
	var records []*core.ApprovalRecord
	if err := s.db.Where("approval_id = ?", approvalID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// List 获取审批记录列表（带分页）
func (s *approvalRecordStore) List(ctx context.Context, offset, limit int) ([]*core.ApprovalRecord, error) {
	var records []*core.ApprovalRecord
	if err := s.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
