package credentials

import (
	"errors"
	"fmt"
)

// WechatWorkType 企业微信应用凭证类型
//
// 用于企业微信应用消息发送，需要配置：
// - corp_id: 企业ID（在「我的企业」中查看）
// - corp_secret: 应用Secret（在应用管理页面查看）
// - agent_id: 应用AgentID（数字类型）
type WechatWorkType struct{}

func (t *WechatWorkType) GetType() string {
	return "wechat_work"
}

func (t *WechatWorkType) GetLabel() string {
	return "企业微信应用"
}

func (t *WechatWorkType) GetIcon() string {
	return "chat"
}

func (t *WechatWorkType) GetDescription() string {
	return "企业微信应用凭证，用于发送应用消息到成员/部门/标签"
}

// GetSecretFields 返回需要加密的敏感字段
// corp_secret 是敏感字段，需要加密存储
func (t *WechatWorkType) GetSecretFields() []string {
	return []string{"corp_secret"}
}

// Validate 验证凭证内容的合法性
func (t *WechatWorkType) Validate(value map[string]interface{}) error {
	// 1. 检查必填字段
	requiredFields := map[string]string{
		"corp_id":     "企业ID",
		"corp_secret": "应用Secret",
		"agent_id":    "应用ID",
	}

	for field, label := range requiredFields {
		if _, ok := value[field]; !ok {
			return fmt.Errorf("%s（%s）不能为空", label, field)
		}
	}

	// 2. 验证 corp_id 格式（以 ww 开头）
	if corpId, ok := value["corp_id"].(string); ok {
		if len(corpId) < 10 {
			return errors.New("企业ID格式不正确（长度过短）")
		}
		// 企业微信的 corp_id 通常以 ww 开头
		// 但不强制校验，因为可能有特殊情况
	}

	// 3. 验证 agent_id 是数字类型
	if agentId, ok := value["agent_id"].(float64); ok {
		if agentId <= 0 {
			return errors.New("应用ID必须大于0")
		}
	} else if _, ok := value["agent_id"].(int); !ok {
		return errors.New("应用ID必须是数字类型")
	}

	return nil
}

// GetDefaultValue 返回默认值示例
func (t *WechatWorkType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{
		"corp_id":     "ww1234567890abcdef",
		"corp_secret": "",
		"agent_id":    1000002,
	}
}

func init() {
	Register(&WechatWorkType{})
}
