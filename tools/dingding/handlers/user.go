package handlers

import (
	"log"

	"cronjob.codelieche/tools/dingding"
	"github.com/kataras/iris"
)

// 用户列表api
func UserListApi(ctx iris.Context) {
	// 定义变量
	var (
		page     int
		pageSize int
		offset   int
		limit    int
		users    []*dingding.User
		err      error
	)

	//	得到page
	page = ctx.Params().GetIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if users, err = dingding.GetUserList(offset, limit); err != nil {
		log.Println(err)
		ctx.HTML("<div>%s</div>", err.Error())
	} else {
		ctx.JSON(users)
	}
}
