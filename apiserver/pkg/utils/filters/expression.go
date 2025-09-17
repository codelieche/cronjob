package filters

import (
	"fmt"
	"gorm.io/gorm/clause"
)

type Contains clause.Like
type IContains clause.Like

func (c Contains) Build(builder clause.Builder) {
	builder.WriteQuoted(c.Column)
	builder.WriteString(" LIKE ")
	builder.AddVar(builder, fmt.Sprintf("%%%s%%", c.Value))
}

func (ic IContains) Build(builder clause.Builder) {
	builder.WriteQuoted(ic.Column)
	builder.WriteString(" ILIKE ")
	builder.AddVar(builder, fmt.Sprintf("%%%s%%", ic.Value))
}
