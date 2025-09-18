package filters

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FILTER_EQ = iota
	FILTER_NEQ
	FILTER_CONTAINS
	FILTER_ICONTAINS
	FILTER_GT
	FILTER_GTE
	FILTER_LT
	FILTER_LTE
	FILTER_LIKE
	FILTER_IN
)

type Filter interface {
	Filter(db *gorm.DB) *gorm.DB
}

type NewClauseExpressionFunc = func(column string, value interface{}) clause.Expression

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
		var values []interface{}
		reflectValue := reflect.ValueOf(value)
		for i := 0; i < reflectValue.Len(); i++ {
			values = append(values, reflectValue.Index(i).Interface())
		}
		return &clause.IN{Column: column, Values: values}
	},
}

type Query interface {
	Query(key string) string
}

type FilterOption struct {
	QueryKey  string
	Column    string
	Value     interface{}
	Op        int
	AllowNull bool
}

func (o *FilterOption) SetValueByQuery(q Query) {
	var queryKey = o.QueryKey
	if o.QueryKey == "" {
		queryKey = o.Column
	}

	var value interface{}
	value = q.Query(queryKey)

	if value != "" && value != nil {
		o.Value = value
	}
}

func (o *FilterOption) ParseExpressionFromQuery(q Query) (c clause.Expression, joinTable string) {
	var queryKey = o.QueryKey
	if o.QueryKey == "" {
		queryKey = o.Column
	}

	var value interface{}
	value = q.Query(queryKey)

	if value == "" && o.Value == nil {
		return nil, ""
	}

	return o.parseExpression(value)
}

func (o *FilterOption) ParseExpression() (c clause.Expression, joinTable string) {

	var value interface{}
	value = o.Value

	if value == "" || value == nil {
		return nil, ""
	}

	return o.parseExpression(value)
}

func (o *FilterOption) parseExpression(value interface{}) (c clause.Expression, joinTable string) {
	// 如果不允许NULL值，且值为空或为nil，则返回空表达式
	if !o.AllowNull && (value == "" || value == nil) {
		return nil, ""
	}

	if newClauseExpressionFunc, exist := ClauseExpressionMap[o.Op]; exist {
		if o.Column != "" {
			if strings.Index(o.Column, ".") > 0 {
				subs := strings.Split(o.Column, ".")
				joinTable = subs[0]
			}
			return newClauseExpressionFunc(o.Column, value), joinTable
		}
	}

	return nil, ""
}

func HandlerQueryFilters(query *gorm.DB, q Query, opts ...*FilterOption) *gorm.DB {
	if opts != nil && len(opts) > 0 {
		tablesLock := map[string]bool{}
		//cList := []clause.Expression{}
		for _, opt := range opts {
			var c clause.Expression
			var table string

			if q != nil {
				c, table = opt.ParseExpressionFromQuery(q)
			} else {
				c, table = opt.ParseExpression()
			}

			if c != nil {
				if table != "" {
					if _, exist := tablesLock[table]; !exist {
						query = query.Joins(table)
						tablesLock[table] = true
					}
				}
				query = query.Clauses(c)
			}
		}
	}
	return query
}

func (o *FilterOption) Filter(db *gorm.DB) *gorm.DB {
	// 解析表达式
	c, table := o.ParseExpression()
	if c != nil {
		if table != "" {
			db = db.Joins(table)
		}
		db = db.Clauses(c)
	}
	return db
}

// func NewFilterOption(column string, value interface{}, op int) Filter {
// 	return &FilterOption{
// 		Column: column,
// 		Value:  value,
// 		Op:     op,
// 	}
// }
