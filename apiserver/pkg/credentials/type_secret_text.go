package credentials

import (
	"errors"
)

// SecretTextType 秘密文本类型
type SecretTextType struct{}

func (t *SecretTextType) GetType() string {
	return "secret_text"
}

func (t *SecretTextType) GetLabel() string {
	return "秘密文本"
}

func (t *SecretTextType) GetIcon() string {
	return "📝"
}

func (t *SecretTextType) GetDescription() string {
	return "任意秘密文本（密码、密钥、证书等）"
}

func (t *SecretTextType) GetSecretFields() []string {
	return []string{"secret"}
}

func (t *SecretTextType) Validate(value map[string]interface{}) error {
	if _, ok := value["secret"]; !ok {
		return errors.New("秘密文本不能为空")
	}
	return nil
}

func (t *SecretTextType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

func init() {
	Register(&SecretTextType{})
}
