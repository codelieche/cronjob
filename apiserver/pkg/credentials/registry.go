package credentials

import (
	"fmt"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
)

// 全局凭证类型注册表
var registry = make(map[string]core.CredentialCategory)

// Register 注册凭证类型
func Register(category core.CredentialCategory) {
	registry[category.GetType()] = category
}

// Get 获取凭证类型实例
func Get(category string) (core.CredentialCategory, error) {
	cat, ok := registry[category]
	if !ok {
		return nil, fmt.Errorf("unsupported credential category: %s", category)
	}
	return cat, nil
}

// GetAll 获取所有已注册的凭证类型
func GetAll() []core.CredentialCategory {
	categories := make([]core.CredentialCategory, 0, len(registry))
	for _, category := range registry {
		categories = append(categories, category)
	}
	return categories
}
