package controllers

import (
	"net/http"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/controllers"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ApprovalRecordController 审批记录控制器
type ApprovalRecordController struct {
	controllers.BaseController
	store core.ApprovalRecordStore
}

// NewApprovalRecordController 创建ApprovalRecordController实例
func NewApprovalRecordController(store core.ApprovalRecordStore) *ApprovalRecordController {
	return &ApprovalRecordController{
		store: store,
	}
}

// List 获取审批记录列表
// @Summary 获取审批记录列表
// @Description 获取审批操作历史记录，支持按审批ID过滤
// @Tags approval-records
// @Accept json
// @Produce json
// @Param approval_id query string false "审批ID（过滤条件）"
// @Param page query int false "页码（默认1）"
// @Param page_size query int false "每页数量（默认20）"
// @Success 200 {object} object "审批记录列表"
// @Router /approval-records/ [get]
// @Security BearerAuth
func (ctrl *ApprovalRecordController) List(c *gin.Context) {
	// 1. 检查是否按审批ID过滤
	approvalIDStr := c.Query("approval_id")
	if approvalIDStr != "" {
		// 按审批ID获取记录列表
		approvalID, err := uuid.Parse(approvalIDStr)
		if err != nil {
			ctrl.HandleError(c, err, http.StatusBadRequest)
			return
		}

		records, err := ctrl.store.FindByApprovalID(c.Request.Context(), approvalID)
		if err != nil {
			ctrl.HandleError(c, err, http.StatusInternalServerError)
			return
		}

		// 返回列表格式（兼容前端分页响应格式）
		ctrl.HandleOK(c, gin.H{
			"count":    len(records),
			"results":  records,
			"page":     1,
			"pageSize": len(records),
		})
		return
	}

	// 2. 获取分页参数
	pagination := ctrl.ParsePagination(c)
	offset := (pagination.Page - 1) * pagination.PageSize

	// 3. 获取审批记录列表
	records, err := ctrl.store.List(c.Request.Context(), offset, pagination.PageSize)
	if err != nil {
		ctrl.HandleError(c, err, http.StatusInternalServerError)
		return
	}

	// 4. 返回分页响应
	ctrl.HandleOK(c, gin.H{
		"count":    len(records), // 注意：这里返回当前页的数量，实际应该查询总数
		"results":  records,
		"page":     pagination.Page,
		"pageSize": pagination.PageSize,
	})
}
