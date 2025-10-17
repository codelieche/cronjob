package controllers

import (
	"github.com/codelieche/todolist/pkg/utils/types"
)

// pageConfig 全局分页配置
// 用于存储分页相关的配置参数，如最大页数、每页最大大小等
var pageConfig *types.PaginationConfig

// SetPaginationConfig 设置分页配置
// 允许在运行时动态修改分页参数，用于不同环境下的配置调整
// 参数: config - 分页配置对象
func SetPaginationConfig(config *types.PaginationConfig) {
	pageConfig = config
}

// init 包初始化函数
// 设置默认的分页配置参数
func init() {
	defaultPaginationConfig := &types.PaginationConfig{
		MaxPage:            1000,        // 最大页数限制，防止恶意请求
		PageQueryParam:     "page",      // 页码查询参数名
		MaxPageSize:        500,         // 每页最大数据量，防止性能问题
		PageSizeQueryParam: "page_size", // 每页大小查询参数名
	}

	SetPaginationConfig(defaultPaginationConfig)
}
