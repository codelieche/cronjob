package controllers

import (
	"fmt"
	"time"

	"github.com/kataras/iris"
)

type IndexController struct {
}

func (c *IndexController) Get(ctx iris.Context) {
	//log.Println(ctx.Path())
	username, password, _ := ctx.Request().BasicAuth()
	//log.Println(ctx.Path(), username, password)
	//ctx.Writef("%s %s %s", ctx.Path(), username, password)
	msg := fmt.Sprintf("%s %s %s", ctx.Path(), username, password)

	ctx.JSON(iris.Map{
		"msg":   msg,
		"time":  time.Now(),
		"other": "Index Controller",
	})
}
