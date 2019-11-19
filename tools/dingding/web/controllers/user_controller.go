package controllers

import (
	"github.com/kataras/iris"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
)

type UserController struct {
	Service services.UserService
}

func (c *UserController) GetBy(idOrName string) (user *datamodels.User, found bool) {
	return c.Service.GetByIdOrName(idOrName) // it will throw 404 if not found.
}

func (c *UserController) GetList(ctx iris.Context) (users []*datamodels.User, success bool) {
	return c.GetListBy(ctx, 1)
}

func (c *UserController) GetListBy(ctx iris.Context, page int) (users []*datamodels.User, success bool) {
	// 定义变量
	var (
		pageSize int
		offset   int
		limit    int
		//err      error
	)

	//	得到page
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize

	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if users, success = c.Service.List(offset, limit); success != true {
		return nil, success
	} else {
		return users, true
	}
}

// 获取用户消息列表
func (c *UserController) GetByMessageList(ctx iris.Context, idOrName string) (messages []*datamodels.Message, success bool) {
	// 定义变量
	var (
		page     int
		pageSize int
		offset   int
		limit    int
		user     *datamodels.User
		//err      error
	)

	//	得到page
	page = ctx.URLParamIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize

	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if user, success = c.Service.GetByIdOrName(idOrName); success != true {
		return nil, success
	}

	//	开始获取用户消息
	if messages, success = c.Service.GetMessageList(user, offset, limit); success {
		return messages, true
	} else {
		return nil, false
	}
}
