package controllers

import (
	"net/http"
	"strconv"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
	"github.com/gin-gonic/gin"
)

type BaseController struct {
}

func (controller *BaseController) HandleOK(c *gin.Context, data interface{}) {
	r := types.Response{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
	c.JSON(http.StatusOK, r)
}

func (controller *BaseController) HandleNoContent(c *gin.Context) {
	c.Writer.WriteHeader(http.StatusNoContent)
}

func (controller *BaseController) SetAuditLog(c *gin.Context, key string, data interface{}, marsharl bool) {
	r := types.Response{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
	c.JSON(http.StatusCreated, r)
}

func (controller *BaseController) HandleCreated(c *gin.Context, data interface{}) {
	r := types.Response{
		Code:    0,
		Data:    data,
		Message: "ok",
	}
	c.JSON(http.StatusCreated, r)
}

func (controller *BaseController) HandleError(c *gin.Context, err error, code int) {
	if err == core.ErrNotFound {
		controller.Handle404(c, err)
		return
	}

	r := types.Response{
		Code:    code,
		Message: err.Error(),
	}

	c.JSON(code, r)
}

func (controller *BaseController) HandleError400(c *gin.Context, err error) {
	if err == core.ErrNotFound {
		controller.Handle404(c, err)
		return
	}

	r := types.Response{
		Code:    http.StatusBadRequest,
		Message: err.Error(),
	}

	c.JSON(http.StatusBadRequest, r)
}

// Handle401 响应401错误
func (controller *BaseController) Handle401(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusUnauthorized,
		Message: err.Error(),
	}
	c.JSON(http.StatusUnauthorized, r)
}

func (controller *BaseController) Handle404(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusNotFound,
		Message: err.Error(),
	}
	c.JSON(http.StatusNotFound, r)
}

func (controller *BaseController) HandleError500(c *gin.Context, err error) {
	r := types.Response{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
	}
	c.JSON(http.StatusInternalServerError, r)
}

func (controller *BaseController) ParsePagination(c *gin.Context) *types.Pagination {
	pageStr := c.DefaultQuery(pageConfig.PageQueryParam, "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1
	}
	if pageConfig.MaxPage > 0 && page > pageConfig.MaxPage {
		page = pageConfig.MaxPage
	}

	pageSizeStr := c.DefaultQuery(pageConfig.PageSizeQueryParam, "10")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		pageSize = 10
	}

	if pageConfig.MaxPageSize > 0 && pageSize > pageConfig.MaxPageSize {
		pageSize = pageConfig.MaxPageSize
	}

	return &types.Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

func (controller *BaseController) FilterAction(
	c *gin.Context, filterOptions []*filters.FilterOption,
	searchFields []string, orderingFields []string, defaultOrdering string) (filterActions []filters.Filter) {

	filterAction := filters.FromQueryGetFilterAction(c, filterOptions)
	if filterAction != nil {
		filterActions = append(filterActions, filterAction)
	}

	searchAction := filters.FromQueryGetSearchAction(c, searchFields)
	if searchAction != nil {
		filterActions = append(filterActions, searchAction)
	}

	var orderingAction filters.Filter
	if orderingFields != nil && defaultOrdering != "" {
		orderingAction = filters.FromQueryGetOrderingActionWithDefault(c, orderingFields, defaultOrdering)
	} else {
		orderingAction = filters.FromQueryGetOrderingAction(c, orderingFields)
	}
	if orderingAction != nil {
		filterActions = append(filterActions, orderingAction)
	}

	return filterActions
}
