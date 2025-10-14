package core

// CredentialCategory 凭证类型接口
// 每个凭证类型（username_password、api_token等）需要实现此接口
type CredentialCategory interface {
	// GetType 获取凭证类型标识（与数据库category字段对应）
	GetType() string

	// GetLabel 获取显示名称
	GetLabel() string

	// GetIcon 获取图标
	GetIcon() string

	// GetDescription 获取描述
	GetDescription() string

	// GetSecretFields 获取需要加密的字段列表
	GetSecretFields() []string

	// Validate 验证凭证值是否合法
	Validate(value map[string]interface{}) error

	// GetDefaultValue 获取默认凭证值
	GetDefaultValue() map[string]interface{}
}
