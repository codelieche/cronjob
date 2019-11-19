package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/repositories"
	"github.com/kataras/iris"
)

type RsyncController struct {
}

func (c *RsyncController) Get(ctx iris.Context) {
	// 执行同步操作
	ding := common.NewDing()
	if err := repositories.RsyncDingDingData(ding); err != nil {
		http.Error(ctx.ResponseWriter(), err.Error(), 500)
		return
	} else {
		ctx.WriteString("同步钉钉数据成功")
	}
}
