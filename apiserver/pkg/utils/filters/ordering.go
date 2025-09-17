package filters

import (
	"fmt"
	"gorm.io/gorm"
	"log"
	"regexp"
	"strings"
)

const OrderingParam string = "ordering"

var orderingRegMatch = regexp.MustCompile("^([\\-])?([\\w][\\w\\d\\-\\_]*)$")

type Ordering struct {
	Fields []string
	query  Query
	Value  string
}

func NewOrdering(fields []string, value string, query Query) Filter {
	return &Ordering{Fields: fields, Value: value, query: query}
}

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

func (o Ordering) Filter(db *gorm.DB) *gorm.DB {
	value := o.Value
	if o.query != nil {
		value = o.query.Query(OrderingParam)
	}
	if value == "" || o.Fields == nil || len(o.Fields) < 1 {
		return db
	}
	parts := strings.Split(value, ",")
	if len(parts) < 1 {
		return db
	}

	ordering := ""
	for _, field := range parts {
		if o.inFields(field) {
			items := orderingRegMatch.FindAllStringSubmatch(field, -1)
			if len(items) == 1 && len(items[0]) == 3 {
				order := items[0][1]
				fieldName := items[0][2]

				if fieldName != "" {
					if ordering == "" {
						if order == "-" {
							ordering = fmt.Sprintf("%s desc", fieldName)
						} else {
							ordering = fieldName
						}
					} else {
						if order == "-" {
							ordering = fmt.Sprintf("%s, %s desc", ordering, fieldName)
						} else {
							ordering = fmt.Sprintf("%s, %s", ordering, fieldName)
						}
					}
				}
			} else {
				log.Println("无需排序", field, items)
			}
		}

	}

	if ordering != "" {
		db = db.Order(ordering)
	}
	return db
}

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

func FromQueryGetOrderingActionWithDefault(q Query, fields []string, value string) (action *Ordering) {
	ordering := q.Query(OrderingParam)

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
