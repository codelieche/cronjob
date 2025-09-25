package filters

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FilterAction 过滤器动作结构体
// 将多个FilterOption组合成一个过滤器动作，统一处理多个过滤条件
type FilterAction struct {
	Options []*FilterOption // 过滤器选项列表
	query   Query           // 查询接口，用于从HTTP请求中获取参数
}

// NewFilterAction 创建新的过滤器动作
// 返回实现了Filter接口的FilterAction实例
func NewFilterAction(options []*FilterOption, query Query) Filter {
	return &FilterAction{Options: options, query: query}
}

// Filter 实现Filter接口，将多个过滤条件应用到GORM查询中
// 批量处理多个FilterOption，自动处理表关联和条件组合
func (f *FilterAction) Filter(db *gorm.DB) *gorm.DB {
	if f.Options == nil || len(f.Options) < 1 {
		return db
	}

	conds := []clause.Expression{}  // 存储所有条件表达式
	joinTables := []string{}        // 存储需要JOIN的表名
	tablesLock := map[string]bool{} // 避免重复JOIN同一个表

	// 遍历所有过滤器选项，生成对应的条件表达式
	for _, opt := range f.Options {
		var c clause.Expression
		var table string

		// 根据是否有查询接口选择不同的解析方式
		if f.query != nil {
			c, table = opt.ParseExpressionFromQuery(f.query)
		} else {
			c, table = opt.ParseExpression()
		}

		if c != nil {
			// 如果需要关联表且尚未处理过，记录表名
			if table != "" {
				if _, exist := tablesLock[table]; !exist {
					tablesLock[table] = true
					joinTables = append(joinTables, table)
				}
			}
			// 将条件表达式添加到列表中
			conds = append(conds, c)
		}
	}

	// 如果有条件表达式，则应用到查询中
	if conds != nil && len(conds) > 0 {
		// 先添加所有需要的JOIN
		if joinTables != nil && len(joinTables) > 0 {
			for _, table := range joinTables {
				db = db.Joins(table)
			}
		}
		// 然后添加所有条件
		db = db.Clauses(conds...)
	}

	return db
}

// FromQueryGetFilterAction 从查询参数中创建过滤器动作
// 遍历所有FilterOption，从HTTP请求中提取对应的值，创建FilterAction
func FromQueryGetFilterAction(q Query, opts []*FilterOption) Filter {
	var options []*FilterOption

	if opts != nil && len(opts) > 0 {
		// 遍历所有过滤器选项，从查询参数中设置值
		for _, opt := range opts {
			opt.SetValueByQuery(q)
			// 只有当值不为空时才添加到结果中
			if opt.Value != nil && opt.Value != "" {
				options = append(options, opt)
			}
		}
	}

	// 如果没有有效的过滤器选项，返回nil
	if options == nil || len(options) < 1 {
		return nil
	}

	return &FilterAction{
		Options: options,
	}
}
