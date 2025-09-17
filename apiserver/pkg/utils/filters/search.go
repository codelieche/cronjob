package filters

import (
	"fmt"
	"gorm.io/gorm"
)

const SearchParam string = "search"

type SearchAction struct {
	Fields []string
	Value  interface{}
}

func NewSearchAction(fields []string, value interface{}) Filter {
	return &SearchAction{Fields: fields, Value: value}
}

func (s *SearchAction) Filter(db *gorm.DB) *gorm.DB {
	var query = ""

	if s.Value == nil || s.Value == "" {
		return db
	}

	for _, field := range s.Fields {
		if query == "" {
			query = fmt.Sprintf("%s LIKE '%%%s%%'", field, s.Value)
		} else {
			query = fmt.Sprintf("%s OR %s LIKE '%%%s%%'", query, field, s.Value)
		}
	}
	if query != "" {
		db = db.Where(query)
	}
	return db
}

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
