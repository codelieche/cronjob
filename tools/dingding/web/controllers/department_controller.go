package crontollers

import (
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
	"github.com/kataras/iris"
)

type DepartmentController struct {
	Service services.DepartmentService
}

// 获取部门详情
func (c *DepartmentController) GetBy(ctx iris.Context, id int64) (department *datamodels.Department, success bool) {
	if department, err := c.Service.GetById(id); err != nil {
		return nil, false
	} else {
		return department, true
	}
}

// 获取部门列表
func (c *DepartmentController) GetList(ctx iris.Context) (departments []*datamodels.Department, success bool) {
	return c.GetListBy(ctx, 1)
}

// 获取部门列表
func (c *DepartmentController) GetListBy(ctx iris.Context, page int) (departments []*datamodels.Department, success bool) {
	var (
		pageSize int
		offset   int
		limit    int
		err      error
	)

	//	得到page
	//page = ctx.Params().GetIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize

	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取部门列表
	if departments, err = c.Service.GetList(offset, limit); err != nil {
		//log.Println(err)
		//ctx.HTML("<div>%s</div>", err.Error())
		return nil, false
	} else {
		//ctx.JSON(departments)
		return departments, true
	}
}

// 获取部门用户列表
func (c *DepartmentController) GetByUserList(ctx iris.Context, id int64) (users []*datamodels.User, success bool) {
	var (
		department *datamodels.Department
		page       int
		pageSize   int
		limit      int
		offset     int
		err        error
	)
	// 获取部门
	if department, err = c.Service.GetById(id); err != nil {
		return nil, false
	} else {
		// 获取部门用户列表
	}

	page = ctx.URLParamIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)
	limit = pageSize

	if page > 1 {
		offset = (page - 1) * limit
	}

	// 获取部门的用户
	if users, err = c.Service.GetUserList(department, offset, limit); err != nil {
		return nil, false
	} else {
		return users, true
	}
}
