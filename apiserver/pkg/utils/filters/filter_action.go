package filters

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FilterAction struct {
	Options []*FilterOption
	query   Query
}

func NewFilterAction(options []*FilterOption, query Query) Filter {
	return &FilterAction{Options: options, query: query}
}

func (f *FilterAction) Filter(db *gorm.DB) *gorm.DB {
	if f.Options == nil || len(f.Options) < 1 {
		return db
	}
	conds := []clause.Expression{}
	joinTables := []string{}

	tablesLock := map[string]bool{}
	for _, opt := range f.Options {
		var c clause.Expression
		var table string

		if f.query != nil {
			c, table = opt.ParseExpressionFromQuery(f.query)
		} else {
			c, table = opt.ParseExpression()
		}

		if c != nil {
			if table != "" {
				if _, exist := tablesLock[table]; !exist {
					tablesLock[table] = true
					joinTables = append(joinTables, table)
				}
			}
			conds = append(conds, c)
		}
	}

	if conds != nil && len(conds) > 0 {
		if joinTables != nil && len(joinTables) > 0 {
			for _, table := range joinTables {
				db = db.Joins(table)
			}
		}
		db = db.Clauses(conds...)
	}

	return db
}

func FromQueryGetFilterAction(q Query, opts []*FilterOption) Filter {
	var options []*FilterOption

	if opts != nil && len(opts) > 0 {
		for _, opt := range opts {
			opt.SetValueByQuery(q)
			if opt.Value != nil && opt.Value != "" {
				options = append(options, opt)
			}
		}
	}

	if options == nil || len(options) < 1 {
		return nil
	}
	return &FilterAction{
		Options: options,
	}
}
