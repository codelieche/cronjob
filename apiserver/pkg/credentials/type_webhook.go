package credentials

import (
	"errors"
	"strings"
)

// WebhookType Webhook凭证类型
//
// 用于存储各类Webhook地址（企业微信机器人、飞书机器人、钉钉机器人等）
type WebhookType struct{}

func (t *WebhookType) GetType() string {
	return "webhook"
}

func (t *WebhookType) GetLabel() string {
	return "Webhook"
}

func (t *WebhookType) GetIcon() string {
	return "🔗" // 链接图标
}

func (t *WebhookType) GetDescription() string {
	return "Webhook地址（用于群机器人、第三方集成等）"
}

func (t *WebhookType) GetSecretFields() []string {
	// webhook 地址本身包含密钥，视为敏感信息
	return []string{"webhook"}
}

func (t *WebhookType) Validate(value map[string]interface{}) error {
	// 检查必填字段
	webhook, ok := value["webhook"]
	if !ok {
		return errors.New("webhook地址不能为空")
	}

	// 检查是否为字符串
	webhookStr, ok := webhook.(string)
	if !ok {
		return errors.New("webhook地址必须是字符串")
	}

	// 检查是否为有效的URL
	webhookStr = strings.TrimSpace(webhookStr)
	if webhookStr == "" {
		return errors.New("webhook地址不能为空")
	}

	// 检查是否以 http:// 或 https:// 开头
	if !strings.HasPrefix(webhookStr, "http://") && !strings.HasPrefix(webhookStr, "https://") {
		return errors.New("webhook地址必须以 http:// 或 https:// 开头")
	}

	return nil
}

func (t *WebhookType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

func init() {
	Register(&WebhookType{})
}
