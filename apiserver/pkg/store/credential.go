package store

import (
	"context"
	"errors"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// credentialStore 凭证存储实现
type credentialStore struct {
	db *gorm.DB
}

// NewCredentialStore 创建CredentialStore实例
func NewCredentialStore(db *gorm.DB) core.CredentialStore {
	return &credentialStore{
		db: db,
	}
}

// FindByID 根据ID查找凭证
func (s *credentialStore) FindByID(ctx context.Context, id uuid.UUID) (*core.Credential, error) {
	var credential core.Credential
	if err := s.db.First(&credential, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &credential, nil
}

// Create 创建凭证
func (s *credentialStore) Create(ctx context.Context, credential *core.Credential) (*core.Credential, error) {
	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(credential).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return credential, nil
}

// Update 更新凭证
func (s *credentialStore) Update(ctx context.Context, credential *core.Credential) (*core.Credential, error) {
	// 检查凭证是否存在
	if _, err := s.FindByID(ctx, credential.ID); err != nil {
		return nil, err
	}

	// 更新凭证
	if err := s.db.Save(credential).Error; err != nil {
		return nil, err
	}

	return credential, nil
}

// DeleteByID 删除凭证（软删除）
func (s *credentialStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 检查凭证是否存在
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	// 软删除
	if err := s.db.Delete(&core.Credential{}, "id = ?", id).Error; err != nil {
		return err
	}

	return nil
}

// List 获取凭证列表（带过滤和分页）
func (s *credentialStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.Credential, error) {
	var credentials []*core.Credential
	query := s.db.Model(&core.Credential{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Find(&credentials).Error; err != nil {
		return nil, err
	}

	return credentials, nil
}

// Count 获取凭证总数（带过滤）
func (s *credentialStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var total int64
	query := s.db.Model(&core.Credential{})

	// 应用过滤器
	for _, filterAction := range filterActions {
		query = filterAction.Filter(query)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

// Patch 动态更新凭证字段
func (s *credentialStore) Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	// 检查凭证是否存在
	if _, err := s.FindByID(ctx, id); err != nil {
		return err
	}

	// 动态更新字段
	if err := s.db.Model(&core.Credential{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}
