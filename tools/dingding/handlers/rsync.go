package handlers

import (
	"net/http"

	"cronjob.codelieche/tools/dingding"
	"github.com/kataras/iris"
)

// 同步数据
func RsyncDingdingData(ctx iris.Context) {
	// 执行同步操作
	if err := dingding.RsyncDingDingData(); err != nil {
		http.Error(ctx.ResponseWriter(), err.Error(), 500)
		return
	} else {
		ctx.WriteString("同步钉钉数据成功")
	}
}
