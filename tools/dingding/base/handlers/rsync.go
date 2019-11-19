package handlers

import (
	"net/http"

	"github.com/codelieche/cronjob/tools/dingding/base"
	"github.com/kataras/iris"
)

// 同步数据
func RsyncDingdingData(ctx iris.Context) {
	// 执行同步操作
	if err := base.RsyncDingDingData(); err != nil {
		http.Error(ctx.ResponseWriter(), err.Error(), 500)
		return
	} else {
		ctx.WriteString("同步钉钉数据成功")
	}
}
