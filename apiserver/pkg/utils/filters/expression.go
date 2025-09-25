package filters

import (
	"fmt"

	"gorm.io/gorm/clause"
)

// Contains 包含查询类型
// 基于GORM的Like子句，实现包含查询功能（区分大小写）
type Contains clause.Like

// IContains 不区分大小写包含查询类型
// 基于GORM的Like子句，实现不区分大小写的包含查询功能
type IContains clause.Like

// Build 实现clause.Expression接口的Build方法
// 构建区分大小写的包含查询SQL：column LIKE '%value%'
func (c Contains) Build(builder clause.Builder) {
	builder.WriteQuoted(c.Column)
	builder.WriteString(" LIKE ")
	builder.AddVar(builder, fmt.Sprintf("%%%s%%", c.Value))
}

// Build 实现clause.Expression接口的Build方法
// 构建不区分大小写的包含查询SQL：column ILIKE '%value%'
func (ic IContains) Build(builder clause.Builder) {
	builder.WriteQuoted(ic.Column)
	builder.WriteString(" ILIKE ")
	builder.AddVar(builder, fmt.Sprintf("%%%s%%", ic.Value))
}
