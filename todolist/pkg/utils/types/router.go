package types

// Controller Router Setting

// RouterPathMethodConfig 路由路径、方法的配置
// 这块还需要优化一下，还不够简单易用
type RouterPathMethodConfig struct {
	Path     string `json:"path"`     // 路径
	Method   string `json:"method"`   // 请求的方法：Get、Post、Put、Delete、Any
	Function string `json:"function"` // 处理请求的函数
}
