package credentials

import (
	"errors"
	"strings"
)

// WebAuthType Web认证类型
type WebAuthType struct{}

func (t *WebAuthType) GetType() string {
	return "web_auth"
}

func (t *WebAuthType) GetLabel() string {
	return "Web认证"
}

func (t *WebAuthType) GetIcon() string {
	return "🌐"
}

func (t *WebAuthType) GetDescription() string {
	return "Web服务完整认证（URL+用户名+密码）"
}

func (t *WebAuthType) GetSecretFields() []string {
	return []string{"password"}
}

func (t *WebAuthType) Validate(value map[string]interface{}) error {
	url, ok := value["url"].(string)
	if !ok || url == "" {
		return errors.New("URL不能为空")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errors.New("URL格式不正确，必须以http://或https://开头")
	}

	if _, ok := value["username"]; !ok {
		return errors.New("用户名不能为空")
	}
	if _, ok := value["password"]; !ok {
		return errors.New("密码不能为空")
	}

	return nil
}

func (t *WebAuthType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

func init() {
	Register(&WebAuthType{})
}
