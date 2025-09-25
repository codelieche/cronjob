package core


// Category 分类
type Category struct {
	ID                  uint   `json:"id"`  // 分类ID
	Code                string `json:"code"` // 分类编码，唯一且不为空
	Name                string `json:"name"`                 // 分类名称
	Setup               string `json:"setup"`                // 初始化脚本
	Teardown            string `json:"teardown"`             // 销毁脚本
	Check               string `json:"check"`                // 检查脚本
	Description         string `json:"description"`              // 分类描述
}
