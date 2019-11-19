package handlers

import (
	"log"
	"strconv"

	"github.com/codelieche/cronjob/tools/dingding"
	"github.com/kataras/iris"
)

// 部门列表api
func DepartmentListApi(ctx iris.Context) {
	// 定义变量
	var (
		page        int
		pageSize    int
		offset      int
		limit       int
		departments []*dingding.Department
		err         error
	)

	//	得到page
	page = ctx.Params().GetIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if departments, err = dingding.GetDepartmentList(offset, limit); err != nil {
		log.Println(err)
		ctx.HTML("<div>%s</div>", err.Error())
	} else {
		ctx.JSON(departments)
	}
}

// 获取部门详情By Name OR ID
func GetDepartmentDetail(ctx iris.Context) {
	var (
		nameOrId     string
		departmentId int
		department   *dingding.Department
		err          error
	)

	// 获取传递过来的值
	nameOrId = ctx.Params().Get("name")
	if departmentId, err = strconv.Atoi(nameOrId); err != nil {
		department, err = dingding.GetDepartmentByName(nameOrId)
	} else {
		department, err = dingding.GetDepartmentByid(departmentId)
	}

	// 判断是否出错
	if err != nil {
		ctx.WriteString(err.Error())
	} else {
		ctx.JSON(department)
	}
}
