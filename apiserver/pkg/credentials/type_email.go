package credentials

import (
	"errors"
	"fmt"
)

// EmailType 邮件配置类型
type EmailType struct{}

func (t *EmailType) GetType() string {
	return "email"
}

func (t *EmailType) GetLabel() string {
	return "邮件配置"
}

func (t *EmailType) GetIcon() string {
	return "📧"
}

func (t *EmailType) GetDescription() string {
	return "SMTP邮件服务配置（用于发送通知邮件）"
}

func (t *EmailType) GetSecretFields() []string {
	return []string{"password"}
}

func (t *EmailType) Validate(value map[string]interface{}) error {
	// 检查必填字段（使用前端的字段名：smtp_host, smtp_port）
	requiredFields := map[string]string{
		"smtp_host": "SMTP主机",
		"smtp_port": "SMTP端口",
		"username":  "用户名",
		"password":  "密码",
	}

	for field, label := range requiredFields {
		if _, ok := value[field]; !ok {
			return fmt.Errorf("%s不能为空", label)
		}
	}

	// 检查端口范围
	if port, ok := value["smtp_port"].(float64); ok {
		if port < 1 || port > 65535 {
			return errors.New("端口号必须在1-65535之间")
		}
	}

	return nil
}

func (t *EmailType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{
		"smtp_port": 587,
		"use_tls":   true,
	}
}

func init() {
	Register(&EmailType{})
}
