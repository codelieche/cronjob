package services

import (
	"context"
	"encoding/json"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/credentials"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CredentialService 凭证服务实现
type CredentialService struct {
	store          core.CredentialStore
	encryptService *credentials.EncryptService
}

// NewCredentialService 创建CredentialService实例
func NewCredentialService(store core.CredentialStore) core.CredentialService {
	return &CredentialService{
		store:          store,
		encryptService: credentials.NewEncryptService(),
	}
}

// FindByID 根据ID查找凭证（返回脱敏后的数据）
func (s *CredentialService) FindByID(ctx context.Context, id string) (*core.Credential, error) {
	// 解析UUID
	credentialID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse credential id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	credential, err := s.store.FindByID(ctx, credentialID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find credential by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	// 脱敏处理
	maskedValue, _ := s.encryptService.Mask(credential.Category, credential.Value)
	credential.Value = maskedValue

	return credential, nil
}

// Create 创建凭证
func (s *CredentialService) Create(ctx context.Context, credential *core.Credential) (*core.Credential, error) {
	// 加密敏感字段
	encryptedValue, err := s.encryptService.Encrypt(credential.Category, credential.Value)
	if err != nil {
		logger.Error("encrypt credential value error", zap.Error(err))
		return nil, err
	}
	credential.Value = encryptedValue

	// 创建凭证
	created, err := s.store.Create(ctx, credential)
	if err != nil {
		logger.Error("create credential error", zap.Error(err))
		return nil, err
	}

	// 返回时脱敏
	maskedValue, _ := s.encryptService.Mask(created.Category, created.Value)
	created.Value = maskedValue

	return created, nil
}

// Update 更新凭证
func (s *CredentialService) Update(ctx context.Context, credential *core.Credential) (*core.Credential, error) {
	// 如果Value字段被修改，需要处理加密
	if credential.Value != "" {
		// 🔥 检查是否有敏感字段为 ******（前端脱敏标记），如果有则需要用原值替换
		var valueMap map[string]interface{}
		if err := json.Unmarshal([]byte(credential.Value), &valueMap); err != nil {
			logger.Error("unmarshal credential value error", zap.Error(err))
			return nil, err
		}

		// 获取凭证类型定义
		cat, err := credentials.Get(credential.Category)
		if err != nil {
			logger.Error("get credential category error", zap.Error(err))
			return nil, err
		}

		// 检查敏感字段是否有 ******
		hasPasswordMask := false
		secretFields := cat.GetSecretFields()
		for _, fieldName := range secretFields {
			if value, ok := valueMap[fieldName]; ok {
				if strValue, ok := value.(string); ok && strValue == "******" {
					hasPasswordMask = true
					break
				}
			}
		}

		// 如果有 ******，需要从 Store 获取原始加密数据并解密，然后替换 ******
		if hasPasswordMask {
			originalCredential, err := s.store.FindByID(ctx, credential.ID)
			if err != nil {
				logger.Error("find original credential error", zap.Error(err))
				return nil, err
			}

			// 解密原始凭证的 value
			decryptedOriginalValue, err := s.encryptService.Decrypt(originalCredential.Category, originalCredential.Value)
			if err != nil {
				logger.Error("decrypt original credential value error", zap.Error(err))
				return nil, err
			}

			var originalValueMap map[string]interface{}
			if err := json.Unmarshal([]byte(decryptedOriginalValue), &originalValueMap); err != nil {
				logger.Error("unmarshal original credential value error", zap.Error(err))
				return nil, err
			}

			// 用原始值替换 ******
			for _, fieldName := range secretFields {
				if value, ok := valueMap[fieldName]; ok {
					if strValue, ok := value.(string); ok && strValue == "******" {
						// 用原始值替换
						if originalValue, exists := originalValueMap[fieldName]; exists {
							valueMap[fieldName] = originalValue
						}
					}
				}
			}

			// 重新序列化
			valueJSON, err := json.Marshal(valueMap)
			if err != nil {
				logger.Error("marshal updated credential value error", zap.Error(err))
				return nil, err
			}
			credential.Value = string(valueJSON)
		}

		// 加密处理
		encryptedValue, err := s.encryptService.Encrypt(credential.Category, credential.Value)
		if err != nil {
			logger.Error("encrypt credential value error", zap.Error(err))
			return nil, err
		}
		credential.Value = encryptedValue
	}

	updated, err := s.store.Update(ctx, credential)
	if err != nil {
		logger.Error("update credential error", zap.Error(err), zap.String("id", credential.ID.String()))
		return nil, err
	}

	// 返回时脱敏
	maskedValue, _ := s.encryptService.Mask(updated.Category, updated.Value)
	updated.Value = maskedValue

	return updated, nil
}

// DeleteByID 删除凭证（软删除）
func (s *CredentialService) DeleteByID(ctx context.Context, id string) error {
	// 解析UUID
	credentialID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse credential id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	err = s.store.DeleteByID(ctx, credentialID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("delete credential error", zap.Error(err), zap.String("id", id))
		}
	}
	return err
}

// List 获取凭证列表（带过滤和分页，返回脱敏后的数据）
func (s *CredentialService) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.Credential, error) {
	credentials, err := s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list credentials error", zap.Error(err))
		return nil, err
	}

	// 对所有凭证进行脱敏处理
	for _, credential := range credentials {
		if maskedValue, err := s.encryptService.Mask(credential.Category, credential.Value); err == nil {
			credential.Value = maskedValue
		}
	}

	return credentials, nil
}

// Count 获取凭证总数（带过滤）
func (s *CredentialService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	total, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count credentials error", zap.Error(err))
	}
	return total, err
}

// Patch 动态更新凭证字段
func (s *CredentialService) Patch(ctx context.Context, id string, updates map[string]interface{}) error {
	// 解析UUID
	credentialID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse credential id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 如果更新value字段，需要先加密
	if value, ok := updates["value"]; ok {
		// 先获取凭证，以便知道category类型
		credential, err := s.store.FindByID(ctx, credentialID)
		if err != nil {
			return err
		}

		// 将value转换为JSON字符串进行加密
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		case map[string]interface{}:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				logger.Error("marshal value error", zap.Error(err))
				return err
			}
			valueStr = string(jsonBytes)
		default:
			return core.ErrBadRequest
		}

		// 加密
		encryptedValue, err := s.encryptService.Encrypt(credential.Category, valueStr)
		if err != nil {
			logger.Error("encrypt credential value error", zap.Error(err))
			return err
		}
		updates["value"] = encryptedValue
	}

	err = s.store.Patch(ctx, credentialID, updates)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("patch credential error", zap.Error(err), zap.String("id", id))
		}
	}
	return err
}

// Decrypt 解密凭证（返回解密后的值）
func (s *CredentialService) Decrypt(ctx context.Context, id string) (map[string]interface{}, error) {
	// 解析UUID
	credentialID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse credential id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 查找凭证
	credential, err := s.store.FindByID(ctx, credentialID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find credential by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	// 解密
	decryptedValue, err := s.encryptService.Decrypt(credential.Category, credential.Value)
	if err != nil {
		logger.Error("decrypt credential value error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	// 解析JSON
	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedValue), &valueMap); err != nil {
		logger.Error("unmarshal credential value error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	return valueMap, nil
}

// DecryptWithMetadata 解密凭证并返回完整信息（包括元数据）
//
// 参数:
//   - ctx: 上下文
//   - id: 凭证ID
//
// 返回值:
//   - map[string]interface{}: 包含完整凭证信息的map
//   - error: 错误信息
func (s *CredentialService) DecryptWithMetadata(ctx context.Context, id string) (map[string]interface{}, error) {
	// 解析UUID
	credentialID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse credential id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	// 查找凭证
	credential, err := s.store.FindByID(ctx, credentialID)
	if err != nil {
		if err != core.ErrNotFound {
			logger.Error("find credential by id error", zap.Error(err), zap.String("id", id))
		}
		return nil, err
	}

	// 解密
	decryptedValue, err := s.encryptService.Decrypt(credential.Category, credential.Value)
	if err != nil {
		logger.Error("decrypt credential value error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	// 解析JSON
	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(decryptedValue), &valueMap); err != nil {
		logger.Error("unmarshal credential value error", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	// 返回完整信息（包括元数据）
	return map[string]interface{}{
		"id":        credential.ID.String(),
		"category":  credential.Category,
		"name":      credential.Name,
		"value":     valueMap,
		"is_active": credential.IsActive,
	}, nil
}
