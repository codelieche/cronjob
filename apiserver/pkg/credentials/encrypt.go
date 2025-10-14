package credentials

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
)

// EncryptService 凭证加密服务
type EncryptService struct {
	crypto *tools.Cryptography
}

// NewEncryptService 创建加密服务实例
func NewEncryptService() *EncryptService {
	return &EncryptService{
		crypto: tools.NewCryptography(types.EncryptionKey),
	}
}

// Encrypt 加密凭证值
// category: 凭证类型（用于确定哪些字段需要加密）
// valueJSON: 凭证值（JSON字符串）
// 返回: 加密后的JSON字符串
func (s *EncryptService) Encrypt(category, valueJSON string) (string, error) {
	// 1. 获取凭证类型实例
	cat, err := Get(category)
	if err != nil {
		return "", err
	}

	// 2. 解析JSON
	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(valueJSON), &valueMap); err != nil {
		return "", fmt.Errorf("invalid JSON format: %w", err)
	}

	// 3. 🔥 数据清理：清理所有字符串字段的前后空格
	valueMap = s.normalizeStringFields(valueMap)

	// 4. 验证（加密前验证）
	if err := cat.Validate(valueMap); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 5. 加密敏感字段
	secretFields := cat.GetSecretFields()

	// 🔥 特殊处理：如果 secretFields 包含 "*"，则加密所有字段
	encryptAll := false
	if len(secretFields) == 1 && secretFields[0] == "*" {
		encryptAll = true
	}

	if encryptAll {
		// 加密所有字段
		for fieldName, value := range valueMap {
			if strValue, ok := value.(string); ok && strValue != "" {
				// 检查是否已经是密文（避免重复加密）
				if isEncrypted, _ := s.crypto.CheckCanDecrypt(strValue); !isEncrypted {
					encrypted, err := s.crypto.Encrypt(strValue)
					if err != nil {
						return "", fmt.Errorf("failed to encrypt field %s: %w", fieldName, err)
					}
					valueMap[fieldName] = encrypted
				}
			}
		}
	} else {
		// 仅加密指定字段
		for _, fieldName := range secretFields {
			if value, ok := valueMap[fieldName]; ok {
				if strValue, ok := value.(string); ok && strValue != "" {
					// 检查是否已经是密文（避免重复加密）
					if isEncrypted, _ := s.crypto.CheckCanDecrypt(strValue); !isEncrypted {
						encrypted, err := s.crypto.Encrypt(strValue)
						if err != nil {
							return "", fmt.Errorf("failed to encrypt field %s: %w", fieldName, err)
						}
						valueMap[fieldName] = encrypted
					}
				}
			}
		}
	}

	// 6. 序列化
	encryptedJSON, err := json.Marshal(valueMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(encryptedJSON), nil
}

// normalizeStringFields 清理凭证数据中所有字符串字段的前后空格
// 🔥 适用场景：
// - username/password: 移除误输入的空格
// - token/api_key: 移除复制粘贴带来的空格
// - url/host: 移除无意义的空格
// - private_key: PEM 格式，trim 安全
//
// 🔥 功能：
// 1. 移除所有字符串类型 value 的前后空格
// 2. 移除 key 的前后空格（适配 key_value 类型）
// 3. 跳过空 key（适配 key_value 类型）
func (s *EncryptService) normalizeStringFields(valueMap map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})

	for key, val := range valueMap {
		// 清理 key：去除前后空格
		trimmedKey := strings.TrimSpace(key)

		// 跳过空 key（主要针对 key_value 类型）
		if trimmedKey == "" {
			continue
		}

		// 清理 value：如果是字符串，去除前后空格
		if strValue, ok := val.(string); ok {
			normalized[trimmedKey] = strings.TrimSpace(strValue)
		} else {
			// 非字符串类型保持原样（如 boolean、number）
			normalized[trimmedKey] = val
		}
	}

	return normalized
}

// Decrypt 解密凭证值
// category: 凭证类型
// valueJSON: 凭证值（JSON字符串，包含密文）
// 返回: 解密后的JSON字符串
func (s *EncryptService) Decrypt(category, valueJSON string) (string, error) {
	cat, err := Get(category)
	if err != nil {
		return "", err
	}

	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(valueJSON), &valueMap); err != nil {
		return "", fmt.Errorf("invalid JSON format: %w", err)
	}

	secretFields := cat.GetSecretFields()

	// 🔥 特殊处理：如果 secretFields 包含 "*"，则解密所有字段
	decryptAll := false
	if len(secretFields) == 1 && secretFields[0] == "*" {
		decryptAll = true
	}

	if decryptAll {
		// 解密所有字段
		for fieldName, value := range valueMap {
			if strValue, ok := value.(string); ok && strValue != "" {
				// 尝试解密
				decrypted, err := s.crypto.Decrypt(strValue)
				if err != nil {
					// 解密失败，可能是明文，保持原样
					continue
				}
				valueMap[fieldName] = decrypted
			}
		}
	} else {
		// 仅解密指定字段
		for _, fieldName := range secretFields {
			if value, ok := valueMap[fieldName]; ok {
				if strValue, ok := value.(string); ok && strValue != "" {
					// 尝试解密
					decrypted, err := s.crypto.Decrypt(strValue)
					if err != nil {
						// 解密失败，可能是明文，保持原样
						continue
					}
					valueMap[fieldName] = decrypted
				}
			}
		}
	}

	decryptedJSON, err := json.Marshal(valueMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(decryptedJSON), nil
}

// Mask 脱敏处理（用于列表显示）
// category: 凭证类型
// valueJSON: 凭证值（JSON字符串）
// 返回: 脱敏后的JSON字符串
func (s *EncryptService) Mask(category, valueJSON string) (string, error) {
	cat, err := Get(category)
	if err != nil {
		return "", err
	}

	var valueMap map[string]interface{}
	if err := json.Unmarshal([]byte(valueJSON), &valueMap); err != nil {
		return "", fmt.Errorf("invalid JSON format: %w", err)
	}

	secretFields := cat.GetSecretFields()

	// 🔥 特殊处理：如果 secretFields 包含 "*"，则脱敏所有字段
	maskAll := false
	if len(secretFields) == 1 && secretFields[0] == "*" {
		maskAll = true
	}

	if maskAll {
		// 脱敏所有字段
		for fieldName := range valueMap {
			valueMap[fieldName] = "******"
		}
	} else {
		// 仅脱敏指定字段
		for _, fieldName := range secretFields {
			if _, ok := valueMap[fieldName]; ok {
				valueMap[fieldName] = "******" // 显示为星号
			}
		}
	}

	maskedJSON, err := json.Marshal(valueMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(maskedJSON), nil
}
