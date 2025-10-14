package credentials

import (
	"errors"
	"fmt"
)

// KeyValueType Key-Value键值对凭证
// 用户可以自定义任意数量的键值对，适用于通用配置场景
type KeyValueType struct{}

// GetType 返回凭证类型编码
func (t *KeyValueType) GetType() string {
	return "key_value"
}

// GetLabel 返回凭证类型显示名称
func (t *KeyValueType) GetLabel() string {
	return "键值对"
}

// GetIcon 返回图标名称
func (t *KeyValueType) GetIcon() string {
	return "list_alt"
}

// GetDescription 返回凭证类型描述
func (t *KeyValueType) GetDescription() string {
	return "通用键值对配置，用户可自定义任意数量的 Key-Value 对（所有 Value 将被加密存储）"
}

// GetSecretFields 返回需要加密的敏感字段列表
// 🔥 关键：返回 "*" 表示所有字段都是敏感的（动态字段场景）
func (t *KeyValueType) GetSecretFields() []string {
	// 特殊标记：返回 ["*"] 表示所有 value 都需要加密
	// 加密服务会自动遍历所有字段进行加密
	return []string{"*"}
}

// Validate 验证凭证内容的合法性
func (t *KeyValueType) Validate(value map[string]interface{}) error {
	// 1. 检查是否至少有一个键值对
	if len(value) == 0 {
		return errors.New("至少需要添加一个键值对")
	}

	// 2. 检查所有 key 是否为非空字符串
	for key, val := range value {
		// 检查 key 不能为空
		if key == "" {
			return errors.New("key 不能为空")
		}

		// 检查 value 必须是字符串类型
		if _, ok := val.(string); !ok {
			return fmt.Errorf("key '%s' 的 value 必须是字符串类型", key)
		}
	}

	return nil
}

// GetDefaultValue 返回默认凭证值（空的键值对）
func (t *KeyValueType) GetDefaultValue() map[string]interface{} {
	// 返回一个示例键值对，方便用户理解使用方式
	return map[string]interface{}{
		"example_key": "example_value",
	}
}

// 自动注册到 Registry
func init() {
	Register(&KeyValueType{})
}
