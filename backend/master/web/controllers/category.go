package controllers

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"github.com/kataras/iris/v12/sessions"
)

type CategoryController struct {
	Session *sessions.Session
	Ctx     iris.Context
	Service services.CategoryService
}

// 根据ID或者Name获取分类
func (c *CategoryController) GetBy(idOrName string) (category *datamodels.Category, success bool) {
	if category, err := c.Service.GetByIdORName(idOrName); err != nil {
		return nil, false
	} else {
		return category, true
	}
}

// 创建分类
func (c *CategoryController) PostCreate(ctx iris.Context) (category *datamodels.Category, err error) {
	// 判断session是否登录

	// 定义变量
	var (
		name          string // 分类的名称
		isActive      string // 是否激活:true 或者 1
		isActiveValue bool   // 是否激活
		checkCmd      string // 分类命令：检查
		setupCmd      string // 分类命令：初始化worker
		tearDownCmd   string // 分类命令：worker退出执行命令
		description   string // 分类的描述说明
	)
	// 获取变量
	// Post传过来的头信息：Content-Type
	contentType := ctx.Request().Header.Get("Content-Type")

	// 判断是否传递的是json
	if strings.Contains(contentType, "application/json") {
		category = &datamodels.Category{}
		if err = ctx.ReadJSON(category); err != nil {
			return nil, err
		}
	} else {
		// 传递的不是application/json
		name = strings.TrimSpace(ctx.FormValue("name"))
		isActive = ctx.FormValue("is_active")
		setupCmd = ctx.FormValue("setup_cmd")
		checkCmd = ctx.FormValue("check_cmd")
		tearDownCmd = ctx.FormValue("tear_down_cmd")
		description = ctx.FormValue("description")

		// 创建为list的分类，路由会有冲突：
		// /api/v1/category/list 这个list到底是获取分类详情，还是分类列表呢？
		if name == "list" {
			err = errors.New("不可创建名字为list的分类")
			return nil, err
		}

		// 先判断是否存在
		if category, err = c.Service.GetByName(name); err != nil {
			if err != common.NotFountError {
				return nil, err
			}
		} else {
			if category.ID > 0 {
				log.Println("分类已经存在")
				return nil, fmt.Errorf("分类已经存在")
			}
		}
		// 创建分类
		if isActive == "true" || isActive == "1" {
			isActiveValue = true
		}
		category = &datamodels.Category{
			EtcdKey:     "",
			Name:        name,
			Description: description,
			CheckCmd:    checkCmd,
			SetupCmd:    setupCmd,
			TearDownCmd: tearDownCmd,
			IsActive:    isActiveValue,
		}
	}

	return c.Service.Create(category)
}

// 更新分类ByName
func (c *CategoryController) PutBy(idOrName string, ctx iris.Context) (category *datamodels.Category, err error) {
	// 判断session是否登录

	// 定义变量
	var (
		name        string // 分类的名称
		isActive    string // 是否激活:true 或者 1
		checkCmd    string // 分类命令：检查
		setupCmd    string // 分类命令：初始化worker
		tearDownCmd string // 分类命令：worker退出执行命令
		description string // 分类的描述说明
	)
	// 获取变量

	name = ctx.FormValue("name")
	name = strings.TrimSpace(name)
	isActive = ctx.FormValue("is_active")
	setupCmd = ctx.FormValue("setup_cmd")
	checkCmd = ctx.FormValue("check_cmd")
	tearDownCmd = ctx.FormValue("tear_down_cmd")
	description = ctx.FormValue("description")

	// 先判断是否存在
	if category, err = c.Service.GetByIdORName(idOrName); err != nil {
		if err == common.NotFountError {
			return nil, err
		} else {
			// 出现其他错误
			return nil, err
		}
	} else {

		if category.ID > 0 {
			// 对name进行校验
			// name是不可修改的
			if name == "" {
				err = errors.New("传递的name值为空")
				return nil, err
			}
			if name != category.Name {
				err = fmt.Errorf("传递的name=%s, 而当前修改的分类的name是%s", name, category.Name)
				return nil, err
			}
			// 分类存在可以修改
			isActive = strings.ToLower(isActive)
			isActive = strings.TrimSpace(isActive)
			if isActive == "1" || isActive == "true" {
				category.IsActive = true
			} else {
				category.IsActive = false
			}
			category.CheckCmd = checkCmd
			category.SetupCmd = setupCmd
			category.TearDownCmd = tearDownCmd
			category.Description = description

			// 保存
			return c.Service.Save(category)
		} else {
			// 分类不存在
			return nil, common.NotFountError
		}
	}
}

// 获取分类的列表
func (c *CategoryController) GetList(ctx iris.Context) (categories []*datamodels.Category, success bool) {
	return c.GetListBy(1, ctx)
}

// 获取分类的列表
func (c *CategoryController) GetListBy(page int, ctx iris.Context) (categories []*datamodels.Category, success bool) {
	// 定义变量
	var (
		pageSize int
		offset   int
		limit    int
		err      error
	)

	// 获取变量
	pageSize = ctx.URLParamIntDefault("pageSize", 10)
	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取分类列表
	if categories, err = c.Service.List(offset, limit); err != nil {
		return nil, false
	} else {
		return categories, true
	}
}

// 根据id或者name删除分类
func (c *CategoryController) DeleteBy(idOrName string) mvc.Result {
	if category, err := c.Service.GetByIdORName(idOrName); err != nil {
		return mvc.Response{
			Code: 400,
			Err:  err,
		}
	} else {
		if category.ID > 0 {
			if err := c.Service.Delete(category); err != nil {
				return mvc.Response{
					Code: 400,
					Err:  err,
				}
			} else {
				return mvc.Response{
					Code: 204,
				}
			}
		} else {
			return mvc.Response{
				Code: 400,
			}
		}
	}
}

func (c *CategoryController) GetByJobsList(idOrName string, ctx iris.Context) (jobs []*datamodels.Job, err error) {
	return c.GetByJobsListBy(idOrName, 1, ctx)
}

func (c *CategoryController) GetByJobsListBy(idOrName string, page int, ctx iris.Context) (jobs []*datamodels.Job, err error) {
	// 定义变量
	var (
		pageSize int
		offset   int
		limit    int
	)
	// 获取变量
	pageSize = ctx.URLParamIntDefault("pageSize", 10)
	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}
	// 获取分类
	if category, err := c.Service.GetByIdORName(idOrName); err != nil {
		return nil, err
	} else {
		// 获取分类的Jobs
		return c.Service.GetJobsList(category, offset, limit)
	}
}
