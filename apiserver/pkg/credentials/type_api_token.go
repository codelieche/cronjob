package credentials

import (
	"errors"
)

// APITokenType API Token类型
type APITokenType struct{}

func (t *APITokenType) GetType() string {
	return "api_token"
}

func (t *APITokenType) GetLabel() string {
	return "API Token"
}

func (t *APITokenType) GetIcon() string {
	return "🎫"
}

func (t *APITokenType) GetDescription() string {
	return "API访问令牌（可用于GitHub、GitLab、云服务等）"
}

func (t *APITokenType) GetSecretFields() []string {
	return []string{"token"}
}

func (t *APITokenType) Validate(value map[string]interface{}) error {
	if _, ok := value["token"]; !ok {
		return errors.New("Token不能为空")
	}
	return nil
}

func (t *APITokenType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

func init() {
	Register(&APITokenType{})
}
