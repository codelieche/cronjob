package filters

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// OrderingParam 排序参数的查询参数名常量
const OrderingParam string = "ordering"

// orderingRegMatch 排序字段的正则表达式
// 匹配格式：可选的"-"前缀 + 字段名（字母开头，可包含字母、数字、下划线、连字符）
var orderingRegMatch = regexp.MustCompile(`^([\-])?([\w][\w\d\-\_]*)$`)

// Ordering 排序结构体
// 实现多字段排序功能，支持升序和降序
type Ordering struct {
	Fields []string // 允许排序的字段列表
	query  Query    // 查询接口，用于从HTTP请求中获取排序参数
	Value  string   // 排序值，格式如："name" 或 "-created_at" 或 "name,-created_at"
}

// NewOrdering 创建新的排序对象
// 返回实现了Filter接口的Ordering实例
func NewOrdering(fields []string, value string, query Query) Filter {
	return &Ordering{Fields: fields, Value: value, query: query}
}

// inFields 检查字段是否在允许排序的字段列表中
// 使用正则表达式验证字段名格式是否正确
func (o Ordering) inFields(field string) bool {
	if o.Fields != nil {
		for _, item := range o.Fields {
			if orderingRegMatch.Match([]byte(item)) {
				return true
			}
		}
		return false
	}
	return false
}

// Filter 实现Filter接口，将排序条件应用到GORM查询中
// 支持多字段排序，字段间用逗号分隔，"-"前缀表示降序
func (o Ordering) Filter(db *gorm.DB) *gorm.DB {
	value := o.Value
	// 如果有查询接口，优先从查询参数中获取排序值
	if o.query != nil {
		value = o.query.Query(OrderingParam)
	}

	// 如果排序值为空或没有允许的字段，直接返回原查询
	if value == "" || o.Fields == nil || len(o.Fields) < 1 {
		return db
	}

	// 按逗号分割多个排序字段
	parts := strings.Split(value, ",")
	if len(parts) < 1 {
		return db
	}

	ordering := ""
	for _, field := range parts {
		// 检查字段是否在允许列表中
		if o.inFields(field) {
			// 使用正则表达式解析字段名和排序方向
			items := orderingRegMatch.FindAllStringSubmatch(field, -1)
			if len(items) == 1 && len(items[0]) == 3 {
				order := items[0][1]     // 排序方向（"-" 或 ""）
				fieldName := items[0][2] // 字段名

				if fieldName != "" {
					if ordering == "" {
						// 第一个字段
						if order == "-" {
							ordering = fmt.Sprintf("`%s` desc", fieldName)
						} else {
							ordering = fmt.Sprintf("`%s`", fieldName)
						}
					} else {
						// 后续字段，用逗号连接
						if order == "-" {
							ordering = fmt.Sprintf("%s, `%s` desc", ordering, fieldName)
						} else {
							ordering = fmt.Sprintf("%s, `%s`", ordering, fieldName)
						}
					}
				}
			} else {
				log.Println("无需排序", field, items)
			}
		}
	}

	// 如果构建了排序条件，则应用到GORM查询中
	if ordering != "" {
		db = db.Order(ordering)
	}
	return db
}

// FromQueryGetOrderingAction 从查询参数中创建排序动作
// 从HTTP请求的查询参数中提取排序信息，创建对应的Ordering对象
func FromQueryGetOrderingAction(q Query, fields []string) (action *Ordering) {
	ordering := q.Query(OrderingParam)

	if ordering != "" {
		action = &Ordering{
			Fields: fields,
			Value:  ordering,
		}
	} else {
		return nil
	}

	// 返回
	return action
}

// FromQueryGetOrderingActionWithDefault 从查询参数中创建排序动作（带默认值）
// 如果查询参数中没有排序信息，则使用提供的默认排序值
func FromQueryGetOrderingActionWithDefault(q Query, fields []string, value string) (action *Ordering) {
	ordering := q.Query(OrderingParam)

	// 如果查询参数中没有排序信息，使用默认值
	if ordering == "" {
		ordering = value
	}

	if ordering != "" {
		action = &Ordering{
			Fields: fields,
			Value:  ordering,
		}
	} else {
		return nil
	}

	return action
}
