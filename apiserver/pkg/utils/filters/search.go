package filters

import (
	"fmt"

	"gorm.io/gorm"
)

// SearchParam 搜索参数的查询参数名常量
const SearchParam string = "search"

// SearchAction 搜索动作结构体
// 实现多字段模糊搜索功能，支持在多个字段中搜索包含指定关键词的记录
type SearchAction struct {
	Fields []string    // 要搜索的字段列表
	Value  interface{} // 搜索关键词
}

// NewSearchAction 创建新的搜索动作
// 返回实现了Filter接口的SearchAction实例
func NewSearchAction(fields []string, value interface{}) Filter {
	return &SearchAction{Fields: fields, Value: value}
}

// Filter 实现Filter接口，将搜索条件应用到GORM查询中
// 使用OR逻辑连接多个字段的LIKE查询，实现多字段模糊搜索
func (s *SearchAction) Filter(db *gorm.DB) *gorm.DB {
	var query = ""

	// 如果搜索值为空，直接返回原查询
	if s.Value == nil || s.Value == "" {
		return db
	}

	// 为每个字段构建LIKE查询条件，使用OR连接
	for _, field := range s.Fields {
		if query == "" {
			// 第一个字段，直接构建查询条件
			query = fmt.Sprintf("%s LIKE '%%%s%%'", field, s.Value)
		} else {
			// 后续字段，使用OR连接
			query = fmt.Sprintf("%s OR %s LIKE '%%%s%%'", query, field, s.Value)
		}
	}

	// 如果构建了查询条件，则应用到GORM查询中
	if query != "" {
		db = db.Where(query)
	}
	return db
}

// FromQueryGetSearchAction 从查询参数中创建搜索动作
// 从HTTP请求的查询参数中提取搜索关键词，创建对应的SearchAction
func FromQueryGetSearchAction(q Query, fields []string) (searchAction *SearchAction) {
	search := q.Query(SearchParam)
	if search != "" {
		searchAction = &SearchAction{
			Fields: fields,
			Value:  search,
		}
	} else {
		return nil
	}

	return searchAction
}
