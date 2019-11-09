package middlewares

import (
	"log"

	"github.com/kataras/iris"
)

// 每次请求打印请求地址和方法
func PrintRequestUrl(ctx iris.Context) {
	log.Println(ctx.Method(), ctx.Request().URL)
	ctx.Next()
}
