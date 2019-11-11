package handlers

import (
	"log"

	"github.com/codelieche/cronjob/tools/dingding"
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

// 获取用户详情
func GetUserDetail(ctx iris.Context) {
	var (
		userID string
		user   *dingding.User
		err    error
	)

	userID = ctx.Params().Get("id")

	if user, err = dingding.GetUserByid(userID); err != nil {
		if err == dingding.NotFountError {
			// 通过名字再查找一次
			if user, err = dingding.GetUserByName(userID); err != nil {
				ctx.WriteString(err.Error())
			} else {
				ctx.JSON(user)
			}
		}
	} else {
		ctx.JSON(user)
	}
}
