package core

// Credential 凭证信息（Worker端使用）
//
// Worker从API Server获取凭证时使用的数据结构。
// 注意：这是解密后的明文数据，仅在内存中使用，不会持久化。
//
// 使用场景：
// - MessageRunner 获取邮件/钉钉配置
// - DatabaseRunner 获取数据库连接信息
// - HTTPRunner 获取 API Token
type Credential struct {
	ID          string                 `json:"id"`                    // 凭证ID（UUID）
	Category    string                 `json:"category"`              // 凭证类型（username_password/api_token/email等）
	Name        string                 `json:"name"`                  // 凭证名称（用于日志显示）
	Value       map[string]interface{} `json:"value"`                 // 凭证内容（已解密的明文）
	Description string                 `json:"description,omitempty"` // 凭证描述（可选）
	IsActive    bool                   `json:"is_active"`             // 是否启用
}

// GetString 从凭证值中获取字符串字段
//
// 参数：
//   - key: 字段名称
//
// 返回值：
//   - string: 字段值（不存在返回空字符串）
//   - bool: 是否存在该字段
//
// 使用示例：
//
//	username, ok := cred.GetString("username")
//	if !ok {
//	    return errors.New("凭证缺少 username 字段")
//	}
func (c *Credential) GetString(key string) (string, bool) {
	if c.Value == nil {
		return "", false
	}

	value, ok := c.Value[key]
	if !ok {
		return "", false
	}

	str, ok := value.(string)
	return str, ok
}

// GetInt 从凭证值中获取整数字段
//
// 参数：
//   - key: 字段名称
//
// 返回值：
//   - int: 字段值（不存在返回0）
//   - bool: 是否存在该字段
//
// 使用示例：
//
//	port, ok := cred.GetInt("port")
//	if !ok {
//	    port = 587  // 使用默认值
//	}
func (c *Credential) GetInt(key string) (int, bool) {
	if c.Value == nil {
		return 0, false
	}

	value, ok := c.Value[key]
	if !ok {
		return 0, false
	}

	// 尝试float64转int（JSON解析数字默认是float64）
	if floatVal, ok := value.(float64); ok {
		return int(floatVal), true
	}

	// 尝试直接int
	if intVal, ok := value.(int); ok {
		return intVal, true
	}

	return 0, false
}

// GetBool 从凭证值中获取布尔字段
//
// 参数：
//   - key: 字段名称
//
// 返回值：
//   - bool: 字段值（不存在返回false）
//   - bool: 是否存在该字段
//
// 使用示例：
//
//	useTLS, ok := cred.GetBool("use_tls")
//	if !ok {
//	    useTLS = true  // 使用默认值
//	}
func (c *Credential) GetBool(key string) (bool, bool) {
	if c.Value == nil {
		return false, false
	}

	value, ok := c.Value[key]
	if !ok {
		return false, false
	}

	boolVal, ok := value.(bool)
	return boolVal, ok
}

// MustGetString 获取必填的字符串字段（不存在则panic）
//
// 参数：
//   - key: 字段名称
//
// 返回值：
//   - string: 字段值
//
// 使用示例：
//
//	username := cred.MustGetString("username")  // 不存在会panic
func (c *Credential) MustGetString(key string) string {
	value, ok := c.GetString(key)
	if !ok {
		panic("凭证缺少必填字段: " + key)
	}
	return value
}

// MustGetInt 获取必填的整数字段（不存在则panic）
//
// 参数：
//   - key: 字段名称
//
// 返回值：
//   - int: 字段值
func (c *Credential) MustGetInt(key string) int {
	value, ok := c.GetInt(key)
	if !ok {
		panic("凭证缺少必填字段: " + key)
	}
	return value
}
