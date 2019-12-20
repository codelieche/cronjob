package controllers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codelieche/cronjob/backend/common"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/mvc"
	"github.com/kataras/iris/v12/sessions"
)

type JobController struct {
	Session *sessions.Session
	Ctx     iris.Context
	Service services.JobService
}

// 根据ID获取分类
func (c *JobController) GetBy(id int64) (job *datamodels.Job, success bool) {
	if job, err := c.Service.GetByID(id); err != nil {
		return nil, false
	} else {
		return job, true
	}
}

// 创建Job
func (c *JobController) PostCreate(ctx iris.Context) (job *datamodels.Job, err error) {
	// 判断session是否登录

	// 定义变量
	var (
		name                                    string // Job的名字
		jobCategory                             *datamodels.Category
		category, timeStr, command, description string
		isActive, saveOutput                    string
		isActiveValue, saveOutputValue          bool
	)

	// 解析POST表单
	name = strings.TrimSpace(ctx.FormValue("name"))
	category = strings.TrimSpace(ctx.FormValue("category"))
	timeStr = ctx.FormValue("time")
	command = ctx.FormValue("command")
	description = ctx.FormValue("description")
	isActive = strings.ToLower(strings.TrimSpace(ctx.FormValue("is_active")))
	saveOutput = strings.ToLower(strings.TrimSpace(ctx.FormValue("save_output")))

	// 先判断分类是否存在
	if category == "" {
		err = errors.New("category不可为空")
		return nil, err
	}
	if jobCategory, err = c.Service.GetCategoryByIDOrName(category); err != nil {
		err = fmt.Errorf("分类(%s): %s", category, err.Error())
		return nil, err
	}

	if name == "" {
		nowStr := time.Now().Format("20060102150405")
		name = fmt.Sprintf("Job:%s-%s", category, nowStr)

	}

	if isActive == "1" || isActive == "true" {
		isActiveValue = true
	}

	if saveOutput == "1" || saveOutput == "true" {
		saveOutputValue = true
	}

	// 创建Job
	job = &datamodels.Job{
		EtcdKey:  "",
		Category: jobCategory,
		//CategoryID:  0,
		Name:        name,
		Time:        timeStr,
		Command:     command,
		Description: description,
		IsActive:    isActiveValue,
		SaveOutput:  saveOutputValue,
	}

	return c.Service.Create(job)
}

// 更新Job
// 为了方便管理，Job的分类是不可修改的
func (c *JobController) PutBy(id int64, ctx iris.Context) (job *datamodels.Job, err error) {
	// 判断session是否登录

	// 定义变量
	var (
		name                           string // Job的名字
		jobCategory                    *datamodels.Category
		time, command, description     string
		isActive, saveOutput           string
		isActiveValue, saveOutputValue bool
		updateFields                   map[string]interface{}
	)

	// 判断job是否存在
	if job, err = c.Service.GetByID(id); err != nil {
		if err == common.NotFountError {
			return nil, err
		} else {
			// 出现其他错误
			return nil, err
		}
	}

	// job存在，开始处理结果

	// 解析PUT表单
	name = strings.TrimSpace(ctx.FormValue("name"))
	//category = strings.TrimSpace(ctx.FormValue("category"))
	time = ctx.FormValue("time")
	command = ctx.FormValue("command")
	description = ctx.FormValue("description")
	isActive = strings.ToLower(strings.TrimSpace(ctx.FormValue("is_active")))
	saveOutput = strings.ToLower(strings.TrimSpace(ctx.FormValue("save_output")))

	// 先判断分类是否存在
	// 分类不做修改
	//if category == "" {
	//	err = errors.New("category不可为空")
	//	return nil, err
	//}
	//if jobCategory, err = c.Service.GetCategoryByIDOrName(category); err != nil {
	//	err = fmt.Errorf("分类(%s): %s", category, err.Error())
	//	return nil, err
	//}

	if isActive == "1" || isActive == "true" {
		isActiveValue = true
	}

	if saveOutput == "1" || saveOutput == "true" {
		saveOutputValue = true
	}

	// 待优化
	updateFields = make(map[string]interface{})
	if job.IsActive != isActiveValue {
		updateFields["IsActive"] = isActiveValue
	}
	if job.SaveOutput != saveOutputValue {
		updateFields["SaveOutput"] = saveOutputValue
	}
	if job.Name != name {
		updateFields["Name"] = name
	}
	if job.Category != jobCategory {
		updateFields["Category"] = jobCategory
	}
	if job.Time != time {
		updateFields["Time"] = time
	}
	if job.Command != command {
		updateFields["Command"] = command
	}
	if job.Description != description {
		updateFields["Description"] = description
	}

	// 对job赋予新的值
	return c.Service.Update(job, updateFields)
}

// 获取Job的列表
func (c *JobController) GetList(ctx iris.Context) (jobs []*datamodels.Job, success bool) {
	return c.GetListBy(1, ctx)
}

// 获取Job的列表
func (c *JobController) GetListBy(page int, ctx iris.Context) (jobs []*datamodels.Job, success bool) {
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
	if jobs, err = c.Service.List(offset, limit); err != nil {
		return nil, false
	} else {
		return jobs, true
	}
}

// 根据ID删除Job
func (c *JobController) DeleteBy(id int64) mvc.Result {
	if job, err := c.Service.GetByID(id); err != nil {
		return mvc.Response{
			Code: 400,
			Err:  err,
		}
	} else {
		if job.ID > 0 {
			// 存在
			if err := c.Service.Delete(job); err != nil {
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
				Code: 404,
			}
		}
	}
}
