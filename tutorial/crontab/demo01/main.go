package main

import (
	"log"

	"github.com/gorhill/cronexpr"
)

func main() {

	// 解析crontab 时间表达式
	// 正确的示例
	if expr, err := cronexpr.Parse("*/5 * * * *"); err != nil {
		log.Panic(err)
	} else {
		log.Println(expr)
	}

	//	MustParse
	expr := cronexpr.MustParse("*/2 * * * *")
	log.Println(expr)

	// 表达式错误的实例
	if expr, err := cronexpr.Parse("*/5 * * * *"); err != nil {
		// syntax error in minute field: '*5'
		log.Panic(err)
	} else {
		log.Println(expr)
	}

}
