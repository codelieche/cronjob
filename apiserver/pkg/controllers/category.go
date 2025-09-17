package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/controllers/forms"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/gin-gonic/gin"
)

// CategoryController 分类控制器
type CategoryController struct {
	controllers.BaseController
	service core.CategoryService
}

// NewCategoryController 创建CategoryController实例
func NewCategoryController(service core.CategoryService) *CategoryController {
	return &CategoryController{
		service: service,
	}
}

// Create 创建分类
func (controller *CategoryController) Create(c *gin.Context) {
	// 1. 处理表单
	var form forms.CategoryCreateForm
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
	category := form.ToCategory()

	// 4. 调用服务创建分类
	createdCategory, err := controller.service.Create(c.Request.Context(), category)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 5. 返回成功响应
	controller.HandleCreated(c, createdCategory)
}

// Find 获取分类信息
func (controller *CategoryController) Find(c *gin.Context) {
	// 1. 获取ID或Code
	idOrCode := c.Param("id")

	// 2. 调用服务通过ID或Code获取分类
	category, err := controller.service.FindByIDOrCode(c.Request.Context(), idOrCode)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 返回分类信息
	controller.HandleOK(c, category)
}

// Update 更新分类信息
func (controller *CategoryController) Update(c *gin.Context) {
	// 1. 获取ID或Code
	idOrCode := c.Param("id")

	// 2. 通过ID或Code获取分类信息
	category, err := controller.service.FindByIDOrCode(c.Request.Context(), idOrCode)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 处理表单数据
	var form forms.CategoryInfoForm
	if err := c.ShouldBind(&form); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 4. 验证表单
	if err := form.Validate(); err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 5. 更新分类信息
	form.UpdateCategory(category)

	// 6. 调用服务更新分类
	updatedCategory, err := controller.service.Update(c.Request.Context(), category)
	if err != nil {
		if err == core.ErrConflict {
			controller.HandleError(c, err, http.StatusConflict)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 7. 返回更新后的分类信息
	controller.HandleOK(c, updatedCategory)
}

// Delete 删除分类
func (controller *CategoryController) Delete(c *gin.Context) {
	// 1. 获取ID或Code
	idOrCode := c.Param("id")

	// 2. 尝试通过ID或Code查找分类
	category, err := controller.service.FindByIDOrCode(c.Request.Context(), idOrCode)
	if err != nil {
		if err == core.ErrNotFound {
			controller.Handle404(c, err)
		} else {
			controller.HandleError(c, err, http.StatusBadRequest)
		}
		return
	}

	// 3. 调用服务删除分类
	err = controller.service.DeleteByID(c.Request.Context(), category.ID)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 3. 返回删除成功
	controller.HandleOK(c, map[string]string{"message": "分类删除成功"})
}

// List 获取分类列表
func (controller *CategoryController) List(c *gin.Context) {
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
			QueryKey: "code",
			Column:   "code",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "code__contains",
			Column:   "code",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "name",
			Column:   "name",
			Op:       filters.FILTER_EQ,
		},
		&filters.FilterOption{
			QueryKey: "name__contains",
			Column:   "name",
			Op:       filters.FILTER_CONTAINS,
		},
		&filters.FilterOption{
			QueryKey: "deleted",
			Column:   "deleted",
			Op:       filters.FILTER_EQ,
		},
	}

	// 3. 定义搜索字段
	searchFields := []string{"code", "name", "description"}

	// 4. 定义排序字段
	orderingFields := []string{"code", "name", "created_at", "updated_at"}
	defaultOrdering := "code"

	// 5. 获取过滤动作
	filterActions := controller.FilterAction(c, filterOptions, searchFields, orderingFields, defaultOrdering)

	// 6. 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 7. 获取分类列表
	categories, err := controller.service.List(c.Request.Context(), offset, pagination.PageSize, filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 8. 获取总数
	count, err := controller.service.Count(c.Request.Context(), filterActions...)
	if err != nil {
		controller.HandleError(c, err, http.StatusBadRequest)
		return
	}

	// 9. 返回列表和分页信息
	controller.HandleOK(c, map[string]interface{}{
		"count":     count,
		"items":     categories,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
	})
}
