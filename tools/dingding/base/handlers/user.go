package handlers

import (
	"log"

	"github.com/codelieche/cronjob/tools/dingding/base"

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
		users    []*base.User
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
	if users, err = base.GetUserList(offset, limit); err != nil {
		log.Println(err)
		ctx.HTML("<div>%s</div>", err.Error())
	} else {
		// log.Println(users)
		ctx.JSON(users)
	}
}

// 获取用户详情
func GetUserDetail(ctx iris.Context) {
	var (
		userID string
		user   *base.User
		err    error
	)

	userID = ctx.Params().Get("id")

	if user, err = base.GetUserByid(userID); err != nil {
		if err == base.NotFountError {
			// 通过名字再查找一次
			if user, err = base.GetUserByName(userID); err != nil {
				ctx.WriteString(err.Error())
			} else {
				ctx.JSON(user)
			}
		}
	} else {
		ctx.JSON(user)
	}
}

// 用户消息列表api
func GetUserMessageListApi(ctx iris.Context) {
	// 定义变量
	var (
		page         int
		pageSize     int
		offset       int
		limit        int
		userIdOrName string
		user         *base.User
		messages     []*base.Message
		err          error
	)

	//	得到url传递的参数：userID, page, pageSize
	userIdOrName = ctx.Params().Get("id")
	page = ctx.URLParamIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if user, err = base.GetUserByid(userIdOrName); err != nil {
		ctx.StatusCode(400)
		ctx.WriteString(err.Error())
		return
	}

	// 获取用户消息
	if messages, err = base.GetUserMessageList(user, offset, limit); err != nil {
		log.Println(err)
		ctx.HTML("<div>%s</div>", err.Error())
	} else {
		// log.Println(users)
		ctx.JSON(messages)
	}
}
