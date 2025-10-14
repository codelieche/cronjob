package credentials

import (
	"errors"
)

// SSHPrivateKeyType SSH私钥类型
type SSHPrivateKeyType struct{}

func (t *SSHPrivateKeyType) GetType() string {
	return "ssh_private_key"
}

func (t *SSHPrivateKeyType) GetLabel() string {
	return "SSH私钥"
}

func (t *SSHPrivateKeyType) GetIcon() string {
	return "🔐"
}

func (t *SSHPrivateKeyType) GetDescription() string {
	return "SSH私钥认证（用于Git、服务器登录等）"
}

func (t *SSHPrivateKeyType) GetSecretFields() []string {
	return []string{"private_key", "passphrase"} // 私钥和密码短语都需要加密
}

func (t *SSHPrivateKeyType) Validate(value map[string]interface{}) error {
	if _, ok := value["private_key"]; !ok {
		return errors.New("私钥不能为空")
	}
	if _, ok := value["username"]; !ok {
		return errors.New("用户名不能为空")
	}
	return nil
}

func (t *SSHPrivateKeyType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

func init() {
	Register(&SSHPrivateKeyType{})
}
