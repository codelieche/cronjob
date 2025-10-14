package forms

import "github.com/google/uuid"

// CredentialForm 凭证表单
type CredentialForm struct {
	TeamID      uuid.UUID              `json:"team_id"`                     // 团队ID（可选，不传则使用当前用户的team_id）
	Category    string                 `json:"category" binding:"required"` // 凭证类型
	Name        string                 `json:"name" binding:"required"`     // 凭证名称
	Description string                 `json:"description"`                 // 凭证描述
	Project     string                 `json:"project"`                     // 项目名称（可选）
	Value       map[string]interface{} `json:"value" binding:"required"`    // 凭证内容
	IsActive    *bool                  `json:"is_active"`                   // 是否启用（可选，默认true）
	Metadata    string                 `json:"metadata"`                    // 元数据
}

// CredentialUpdateForm 凭证更新表单
type CredentialUpdateForm struct {
	Name        string                 `json:"name"`        // 凭证名称
	Description string                 `json:"description"` // 凭证描述
	Project     string                 `json:"project"`     // 项目名称（可选）
	Value       map[string]interface{} `json:"value"`       // 凭证内容
	IsActive    *bool                  `json:"is_active"`   // 是否启用
	Metadata    string                 `json:"metadata"`    // 元数据
}
