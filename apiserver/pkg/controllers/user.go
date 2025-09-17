package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	controllers.BaseController
	service core.UserService
}

func NewUserController(service core.UserService) *UserController {
	return &UserController{
		service: service,
	}
}

// Create 创建用户
func (controller *UserController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.UserCreateForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 2. 对表单进行校验
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 准备创建对象
	user := form.ToUser()

	// 4. 调用服务创建用户
	createdUser, err := controller.service.Create(c.Request.Context(), user)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdUser)
}

// Find 获取用户信息
func (controller *UserController) Find(c *gin.Context) {
	// 1. 获取用户的id
	id := c.Param("id")

	// 2. 调用服务获取用户
	user, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回用户信息
	controller.HandleOK(c, user)
}

// Update 更新用户信息
func (controller *UserController) Update(c *gin.Context) {
	// 1. 获取用户的id
	id := c.Param("id")

	// 2. 获取用户信息
	user, err := controller.service.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单数据
	var form forms.UserInfoForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 更新用户信息
	form.UpdateUser(user)

	// 6. 调用服务更新用户
	updatedUser, err := controller.service.Update(c.Request.Context(), user)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 7. 返回更新后的用户信息
	controller.HandleOK(c, updatedUser)
}

// Delete 删除用户
func (controller *UserController) Delete(c *gin.Context) {
	// 1. 获取用户的id
	id := c.Param("id")

	// 2. 调用服务删除用户
	err := controller.service.DeleteByID(c.Request.Context(), id)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回删除成功
	controller.HandleOK(c, map[string]string{"message": "用户删除成功"})
}

// List 获取用户列表
func (controller *UserController) List(c *gin.Context) {
	// 1. 解析分页参数
	pagination := controller.ParsePagination(c)

	// 2. 定义过滤选项
	filterOptions := []*filters.FilterOption{
		&filters.FilterOption{
			QueryKey: "id",
			Column:   "id",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "id__gte",
			Column:   "id",
			Op:       filters.FILTER_GTE,
		},
		&filters.FilterOption{
			QueryKey: "id__lte",
			Column:   "id",
			Op:       filters.FILTER_LTE,
		},
		&filters.FilterOption{
			QueryKey: "username",
			Column:   "username",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "username__contains",
			Column:   "username",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "phone",
			Column:   "phone",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "email",
			Column:   "email",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "is_active",
			Column:   "is_active",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"username", "nickname", "email", "phone", "description", "comment"}

	// 4. 定义排序字段
	orderingFields := []string{"id", "username", "created_at", "updated_at", "last_login"}
	defaultOrdering := "-id"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取用户列表
	users, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取用户总数
	total, err := controller.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 9. 构建分页结果 - 使用更扁平的响应格式
	result := &types.ResponseList{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Count:    total,
		Results:  users,
	}

	// 10. 返回结果
	controller.HandleOK(c, result)
}
