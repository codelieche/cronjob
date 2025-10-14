package credentials

import (
	"errors"
)

// UsernamePasswordType 用户名+密码类型
type UsernamePasswordType struct{}

func (t *UsernamePasswordType) GetType() string {
	return "username_password"
}

func (t *UsernamePasswordType) GetLabel() string {
	return "用户名+密码"
}

func (t *UsernamePasswordType) GetIcon() string {
	return "🔑"
}

func (t *UsernamePasswordType) GetDescription() string {
	return "通用的用户名密码认证（可用于数据库、Jenkins、Harbor、GitLab等）"
}

func (t *UsernamePasswordType) GetSecretFields() []string {
	return []string{"password"}
}

func (t *UsernamePasswordType) Validate(value map[string]interface{}) error {
	if _, ok := value["username"]; !ok {
		return errors.New("用户名不能为空")
	}
	if _, ok := value["password"]; !ok {
		return errors.New("密码不能为空")
	}
	return nil
}

func (t *UsernamePasswordType) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{}
}

// 注册到全局注册表
func init() {
	Register(&UsernamePasswordType{})
}
