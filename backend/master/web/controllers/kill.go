package controllers

import (
	"strconv"
	"strings"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

type JobKillController struct {
	Session *sessions.Session
	Ctx     iris.Context
	Service services.JobKillService
}

// 根据ID获取JobKill
func (c *JobKillController) GetBy(id int64) (job *datamodels.JobKill, success bool) {
	if jobKill, err := c.Service.GetByID(id); err != nil {
		return nil, false
	} else {
		return jobKill, true
	}
}

// 创建JobKill
func (c *JobKillController) PostCreate(ctx iris.Context) (jobKill *datamodels.JobKill, err error) {
	// 判断session是否登录

	// 定义变量
	var (
		category string
		jobIDStr string
		jobID    int
	)

	// 获取变量
	category = strings.TrimSpace(ctx.FormValue("category"))
	jobIDStr = ctx.FormValue("job_id")

	if jobID, err = strconv.Atoi(jobIDStr); err != nil {
		return nil, err
	}

	// 实例化JobKill
	jobKill = &datamodels.JobKill{
		EtcdKey:    "",
		Category:   category,
		JobID:      uint(jobID),
		Killed:     false,
		FinishedAt: nil,
		Result:     "",
	}

	// 创建
	return c.Service.Create(jobKill)

}

// 获取JobKill的列表
func (c *JobKillController) GetList(ctx iris.Context) (jobKills []*datamodels.JobKill, success bool) {
	return c.GetListBy(1, ctx)
}

func (c *JobKillController) GetListBy(page int, ctx iris.Context) (jobKills []*datamodels.JobKill, success bool) {
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

	// 获取Job列表
	if jobKills, err = c.Service.List(offset, limit); err != nil {
		return nil, false
	} else {
		return jobKills, true
	}
}

// 设置JobKill为完成
func (c *JobKillController) PutByFinished(id int64) (jobKill *datamodels.JobKill, err error) {
	if jobKill, err := c.Service.SetFinishedByID(id); err != nil {
		return nil, err
	} else {
		return jobKill, err
	}
}
