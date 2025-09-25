package filters

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 过滤器操作类型常量定义
// 使用iota自动递增，定义了常用的数据库查询操作符
const (
	FILTER_EQ        = iota // 等于 (=)
	FILTER_NEQ              // 不等于 (!=)
	FILTER_CONTAINS         // 包含 (LIKE '%value%')
	FILTER_ICONTAINS        // 不区分大小写包含 (ILIKE '%value%')
	FILTER_GT               // 大于 (>)
	FILTER_GTE              // 大于等于 (>=)
	FILTER_LT               // 小于 (<)
	FILTER_LTE              // 小于等于 (<=)
	FILTER_LIKE             // 模糊匹配 (LIKE)
	FILTER_IN               // 在列表中 (IN)
)

// Filter 接口定义了所有过滤器必须实现的方法
// 用于将过滤器应用到GORM查询中
type Filter interface {
	Filter(db *gorm.DB) *gorm.DB
}

// NewClauseExpressionFunc 类型别名，定义了创建GORM子句表达式的函数类型
// 接收列名和值，返回对应的GORM子句表达式
type NewClauseExpressionFunc = func(column string, value interface{}) clause.Expression

// ClauseExpressionMap 操作符到GORM子句表达式的映射表
// 将我们定义的常量操作符映射到具体的GORM子句实现
var ClauseExpressionMap = map[int]NewClauseExpressionFunc{
	FILTER_EQ: func(column string, value interface{}) clause.Expression {
		return &clause.Eq{Column: column, Value: value}
	},
	FILTER_NEQ: func(column string, value interface{}) clause.Expression {
		return &clause.Neq{Column: column, Value: value}
	},
	FILTER_CONTAINS: func(column string, value interface{}) clause.Expression {
		return &Contains{Column: column, Value: value}
	},
	FILTER_ICONTAINS: func(column string, value interface{}) clause.Expression {
		return &IContains{Column: column, Value: value}
	},
	FILTER_LIKE: func(column string, value interface{}) clause.Expression {
		return &clause.Like{Column: column, Value: value}
	},
	FILTER_GT: func(column string, value interface{}) clause.Expression {
		return &clause.Gt{Column: column, Value: value}
	},
	FILTER_GTE: func(column string, value interface{}) clause.Expression {
		return &clause.Gte{Column: column, Value: value}
	},
	FILTER_LT: func(column string, value interface{}) clause.Expression {
		return &clause.Lt{Column: column, Value: value}
	},
	FILTER_LTE: func(column string, value interface{}) clause.Expression {
		return &clause.Lte{Column: column, Value: value}
	},
	FILTER_IN: func(column string, value interface{}) clause.Expression {
		// 将切片类型的值转换为interface{}切片，用于IN查询
		var values []interface{}
		reflectValue := reflect.ValueOf(value)
		for i := 0; i < reflectValue.Len(); i++ {
			values = append(values, reflectValue.Index(i).Interface())
		}
		return &clause.IN{Column: column, Values: values}
	},
}

// Query 接口定义了从查询参数中获取值的方法
// 通常由gin.Context实现，用于从HTTP请求参数中提取值
type Query interface {
	Query(key string) string
}

// FilterOption 过滤器选项结构体
// 定义了单个过滤条件的所有必要信息
type FilterOption struct {
	QueryKey  string      // 查询参数名，用于从HTTP请求中获取值
	Column    string      // 数据库列名，用于构建WHERE条件
	Value     interface{} // 过滤值，可以直接设置或从查询参数中获取
	Op        int         // 操作符类型，使用上面定义的常量
	AllowNull bool        // 是否允许空值，false时空值会被忽略
}

// SetValueByQuery 从查询参数中设置过滤值
// 如果QueryKey为空，则使用Column作为查询参数名
func (o *FilterOption) SetValueByQuery(q Query) {
	var queryKey = o.QueryKey
	if o.QueryKey == "" {
		queryKey = o.Column
	}

	var value interface{}
	value = q.Query(queryKey)

	// 只有当值不为空且不为nil时才设置
	if value != "" && value != nil {
		o.Value = value
	}
}

// ParseExpressionFromQuery 从查询参数中解析并生成GORM子句表达式
// 返回生成的表达式和需要JOIN的表名（如果有的话）
func (o *FilterOption) ParseExpressionFromQuery(q Query) (c clause.Expression, joinTable string) {
	var queryKey = o.QueryKey
	if o.QueryKey == "" {
		queryKey = o.Column
	}

	var value interface{}
	value = q.Query(queryKey)

	// 如果查询参数为空且Value也为空，则返回空表达式
	if value == "" && o.Value == nil {
		return nil, ""
	}

	return o.parseExpression(value)
}

// ParseExpression 从已设置的Value中解析并生成GORM子句表达式
// 返回生成的表达式和需要JOIN的表名（如果有的话）
func (o *FilterOption) ParseExpression() (c clause.Expression, joinTable string) {
	var value interface{}
	value = o.Value

	// 如果值为空，则返回空表达式
	if value == "" || value == nil {
		return nil, ""
	}

	return o.parseExpression(value)
}

// parseExpression 内部方法，根据操作符和值生成GORM子句表达式
// 支持表关联查询，如果列名包含"."则提取表名用于JOIN
func (o *FilterOption) parseExpression(value interface{}) (c clause.Expression, joinTable string) {
	// 如果不允许NULL值，且值为空或为nil，则返回空表达式
	if !o.AllowNull && (value == "" || value == nil) {
		return nil, ""
	}

	// 从映射表中获取对应的子句表达式生成函数
	if newClauseExpressionFunc, exist := ClauseExpressionMap[o.Op]; exist {
		if o.Column != "" {
			// 检查列名是否包含表名（如 "users.name"）
			if strings.Index(o.Column, ".") > 0 {
				subs := strings.Split(o.Column, ".")
				joinTable = subs[0] // 提取表名用于后续JOIN操作
			}
			return newClauseExpressionFunc(o.Column, value), joinTable
		}
	}

	return nil, ""
}

// HandlerQueryFilters 处理查询过滤器的通用函数
// 将多个FilterOption应用到GORM查询中，自动处理表关联
func HandlerQueryFilters(query *gorm.DB, q Query, opts ...*FilterOption) *gorm.DB {
	if opts != nil && len(opts) > 0 {
		tablesLock := map[string]bool{} // 用于避免重复JOIN同一个表

		for _, opt := range opts {
			var c clause.Expression
			var table string

			// 根据是否有Query对象选择不同的解析方式
			if q != nil {
				c, table = opt.ParseExpressionFromQuery(q)
			} else {
				c, table = opt.ParseExpression()
			}

			if c != nil {
				// 如果需要关联表且尚未JOIN过，则添加JOIN
				if table != "" {
					if _, exist := tablesLock[table]; !exist {
						query = query.Joins(table)
						tablesLock[table] = true
					}
				}
				// 将生成的子句表达式添加到查询中
				query = query.Clauses(c)
			}
		}
	}
	return query
}

// Filter 实现Filter接口，将单个FilterOption应用到GORM查询中
func (o *FilterOption) Filter(db *gorm.DB) *gorm.DB {
	// 解析表达式
	c, table := o.ParseExpression()
	if c != nil {
		// 如果需要关联表，则添加JOIN
		if table != "" {
			db = db.Joins(table)
		}
		// 将子句表达式添加到查询中
		db = db.Clauses(c)
	}
	return db
}

// 注释掉的构造函数，可以根据需要启用
// func NewFilterOption(column string, value interface{}, op int) Filter {
// 	return &FilterOption{
// 		Column: column,
// 		Value:  value,
// 		Op:     op,
// 	}
// }
