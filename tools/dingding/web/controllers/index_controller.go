package controllers

import (
	"fmt"
	"time"

	"github.com/kataras/iris/sessions"

	"github.com/kataras/iris"
)

type IndexController struct {
	Session   *sessions.Session
	StartTime time.Time
}

func (c *IndexController) Get(ctx iris.Context) {

	// 通过session获取visits
	visits := c.Session.Increment("visits", 1)
	sinces := time.Now().Sub(c.StartTime).Seconds()

	//log.Println(ctx.Path())
	username, password, _ := ctx.Request().BasicAuth()
	//log.Println(ctx.Path(), username, password)
	//ctx.Writef("%s %s %s", ctx.Path(), username, password)
	msg := fmt.Sprintf("%s %s %s", ctx.Path(), username, password)

	ctx.JSON(iris.Map{
		"msg":    msg,
		"time":   time.Now(),
		"other":  "Index Controller",
		"visits": visits,
		"sinces": sinces,
	})
}
