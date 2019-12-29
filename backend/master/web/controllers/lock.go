package controllers

import (
	"fmt"

	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/master/web/services"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

type LockController struct {
	Ctx     *iris.Context
	Session *sessions.Session
	Service services.LockService
}

func (c *LockController) PostCreate(ctx iris.Context) (lock *datamodels.Lock, err error) {
	// 1. 定义变量
	var (
		name string
		ttl  int64
	)

	// 2. 获取变量
	name = ctx.PostValue("name")
	ttl = ctx.PostValueInt64Default("ttl", 10)

	// 3. 发起抢锁请求
	if lock, err = c.Service.Create(name, ttl); err != nil {
		return nil, err
	} else {
		return lock, nil
	}
}

// 对锁进行续租
func (c *LockController) PostLease(ctx iris.Context) {
	// 1. 变量处理
	var (
		leaseID  int64
		password string
		message  string
		response datamodels.BaseResponse
		err      error
	)

	// 2. 获取变量
	leaseID = ctx.PostValueInt64Default("lease_id", 0)
	password = ctx.URLParamDefault("password", "")

	// 2. 发起续租请求
	if err = c.Service.Lease(leaseID, password); err != nil {
		response = datamodels.BaseResponse{
			Status:  "error",
			Message: err.Error(),
		}
	} else {
		message = fmt.Sprintf("租约%d续租成功", leaseID)
		response = datamodels.BaseResponse{
			Status:  "success",
			Message: message,
		}
	}

	// 3. 填写响应数据
	ctx.JSON(response)
	return
}

// 对锁进行是否
func (c *LockController) DeleteReleaseBy(leaseID int64, ctx iris.Context) {
	// 1. 变量处理
	var (
		password string
		message  string
		response datamodels.BaseResponse
		err      error
	)

	// 2. 获取变量
	password = ctx.URLParamDefault("password", "")

	// 2. 发起续租请求
	if err = c.Service.Release(leaseID, password); err != nil {
		response = datamodels.BaseResponse{
			Status:  "error",
			Message: err.Error(),
		}
	} else {
		message = fmt.Sprintf("租约%d释放成功", leaseID)
		response = datamodels.BaseResponse{
			Status:  "success",
			Message: message,
		}
	}

	// 3. 填写响应数据
	ctx.JSON(response)
	return
}
