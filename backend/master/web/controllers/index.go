package controllers

import (
	"time"

	"github.com/kataras/iris/v12/sessions"

	"github.com/kataras/iris/v12"
)

type IndexController struct {
	Ctx       iris.Context
	StartTime time.Time
}

func (c *IndexController) Get(ctx iris.Context) {
	ctx.View("index.html")
}

func (c *IndexController) GetInfo(ctx iris.Context) {
	// 通过session获取visits
	sess := sessions.Get(ctx)
	visits := sess.Increment("visits", 1)
	//visits := sess.GetIntDefault("visits", 1)
	sinces := time.Now().Sub(c.StartTime).Seconds()

	ctx.JSON(iris.Map{
		"path":    ctx.Path(),
		"session": sess.ID(),
		"time":    time.Now(),
		"other":   "Index Controller",
		"visits":  visits,
		"sinces":  sinces,
	})
}

func (c *IndexController) GetPing(ctx iris.Context) {

	session := sessions.Get(ctx)
	session.Set("ping", "pong")

	result := session.Get("ping")

	if result != nil {

	} else {
		// session出问题了
		ctx.StatusCode(500)
	}

	ctx.JSON(
		iris.Map{
			"session": session.ID(),
			"message": result,
		})
}
