package controllers

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"github.com/kataras/iris/v12/sessions"
)

type WorkerController struct {
	Session *sessions.Session
	Ctx     iris.Context
	Service services.WorkerService
}

func (c *WorkerController) GetBy(name string) (worker *datamodels.Worker, success bool) {
	if worker, err := c.Service.Get(name); err != nil {
		return nil, false
	} else {
		return worker, true
	}
}

func (c *WorkerController) PostCreate(ctx iris.Context) (worker *datamodels.Worker, err error) {

	// 1. 从结果中获取worker信息
	worker = &datamodels.Worker{}
	if err = ctx.ReadJSON(worker); err != nil {
		return nil, err
	}

	// 2. 把worker信息插入到数据库中
	return c.Service.Create(worker)
}

func (c *WorkerController) DeleteBy(name string) mvc.Result {
	if success, err := c.Service.DeleteByName(name); err != nil {
		return mvc.Response{
			Err: err,
		}
	} else {
		if success {
			return mvc.Response{
				Code: 204,
			}
		} else {
			return mvc.Response{
				Code: 400,
			}
		}
	}
}

func (c *WorkerController) GetList() (workers []*datamodels.Worker, success bool) {
	if workers, err := c.Service.List(); err != nil {
		return nil, false
	} else {
		return workers, true
	}
}
