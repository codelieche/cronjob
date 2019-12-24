package controllers

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"github.com/kataras/iris/v12/sessions"
)

type JobExecuteController struct {
	Session *sessions.Session
	Ctx     iris.Context
	Service services.JobExecuteService
}

// 根据ID获取JobExecute
func (c *JobExecuteController) GetBy(id int64) (jobExecute *datamodels.JobExecute, success bool) {
	if jobExecute, err := c.Service.GetByID(id); err != nil {
		return nil, false
	} else {
		return jobExecute, true
	}
}

// 根据ID获取JobExecute的日志
func (c *JobExecuteController) GetByLog(id int64) (jobExecuteLog *datamodels.JobExecuteLog, success bool) {
	if jobExecuteLog, err := c.Service.GetExecuteLogByID(id); err != nil {
		return nil, false
	} else {
		return jobExecuteLog, true
	}
}

// 获取列表
func (c *JobExecuteController) GetList(ctx iris.Context) (jobExecutes []*datamodels.JobExecute, success bool) {
	return c.GetListBy(1, ctx)
}

func (c *JobExecuteController) GetListBy(page int, ctx iris.Context) (jobExecutes []*datamodels.JobExecute, success bool) {
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

	// 获取JobExecute
	if jobExecutes, err = c.Service.List(offset, limit); err != nil {
		return nil, false
	} else {
		return jobExecutes, true
	}
}

// 杀掉执行
func (c *JobExecuteController) DeleteByKill(id int64) mvc.Result {
	// 执行kill
	if success, err := c.Service.KillByID(id); err != nil {
		return mvc.Response{
			Code: 400,
			Err:  err,
		}
	} else {
		if success {
			return mvc.Response{
				Code: 204,
			}
		} else {
			return mvc.Response{
				Code: 400,
				Text: "kill失败",
			}
		}
	}
}
