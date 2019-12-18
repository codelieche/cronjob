package controllers

import (
	"fmt"
	"log"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
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
		name        string // 分类的名称
		isActive    string // 是否激活:true 或者 1
		checkCmd    string // 分类命令：检查
		setupCmd    string // 分类命令：初始化worker
		tearDownCmd string // 分类命令：worker退出执行命令
		description string // 分类的描述说明
	)
	// 获取变量
	contentType := ctx.Request().Header.Get("Context-Type")

	name = ctx.FormValue("name")
	isActive = ctx.FormValue("is_active")
	setupCmd = ctx.FormValue("setup_cmd")
	checkCmd = ctx.FormValue("check_cmd")
	tearDownCmd = ctx.FormValue("tear_down_cmd")
	description = ctx.FormValue("description")

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
	log.Println()

	// 创建分类
	category = &datamodels.Category{
		EtcdKey:     "",
		Name:        name,
		Description: description,
		CheckCmd:    checkCmd,
		SetupCmd:    setupCmd,
		TearDownCmd: tearDownCmd,
		IsActive:    true,
	}

	log.Println(contentType, isActive, category)

	return c.Service.Create(category)
}

func (c *CategoryController) GetList(ctx iris.Context) (categories []*datamodels.Category, success bool) {
	return c.GetListBy(1, ctx)
}

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
