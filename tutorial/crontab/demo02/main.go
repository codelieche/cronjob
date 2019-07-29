package main

import (
	"log"
	"time"

	"github.com/gorhill/cronexpr"
)

func main() {

	// 生成个channel，用来阻塞主线程
	c := make(chan bool, 1)

	// 解析crontab 时间表达式
	// 正确的示例
	if expr, err := cronexpr.Parse("*/2 * * * *"); err != nil {
		log.Panic(err)
	} else {
		log.Println(expr)

		//	获取下次执行的时间
		now := time.Now()
		next := expr.Next(now)
		log.Println(next.String())

		// 到这个时间的时候开始执行
		time.AfterFunc(next.Sub(now), func() {
			log.Println("我执行咯：", next)

			// 传递个布尔型的值到channel中
			c <- true
		})

	}

	log.Println("等待取到c中的元素")
	result := <-c
	log.Println(result, time.Now())

	// 关闭channel
	close(c)

	log.Println("=== Done ===")

}
