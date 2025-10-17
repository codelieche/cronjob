package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApprovalRecord 审批操作历史
type ApprovalRecord struct {
	ID         uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	ApprovalID uuid.UUID       `gorm:"type:char(36);not null;index" json:"approval_id"`
	Action     string          `gorm:"type:varchar(20);not null" json:"action"` // approve/reject/comment
	UserID     *uuid.UUID      `gorm:"type:char(36);index" json:"user_id"`
	AIAgentID  *uuid.UUID      `gorm:"type:char(36);index" json:"ai_agent_id"`
	Comment    string          `gorm:"type:text" json:"comment"`
	Metadata   json.RawMessage `gorm:"type:json" json:"metadata"` // 元数据，存储操作来源、IP地址等信息
	CreatedAt  time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ApprovalRecord) TableName() string {
	return "approval_records"
}

// BeforeCreate 创建前生成UUID
func (r *ApprovalRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// ApprovalRecordStore 审批记录存储接口
type ApprovalRecordStore interface {
	// Create 创建审批记录
	Create(ctx context.Context, record *ApprovalRecord) (*ApprovalRecord, error)

	// FindByApprovalID 根据审批ID查找记录列表
	FindByApprovalID(ctx context.Context, approvalID uuid.UUID) ([]*ApprovalRecord, error)

	// List 获取审批记录列表（带分页）
	List(ctx context.Context, offset, limit int) ([]*ApprovalRecord, error)
}
