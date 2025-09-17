package forms

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
)

// WorkerCreateForm 工作节点创建表单
type WorkerCreateForm struct {
	ID          string          `json:"id" form:"id"`
	Name        string          `json:"name" form:"name" binding:"required"`
	Description string          `json:"description" form:"description"`
	IsActive    *bool           `json:"is_active" form:"is_active"`
	Metadata    json.RawMessage `json:"metadata" form:"metadata"`
}

// Validate 验证表单
func (form *WorkerCreateForm) Validate() error {
	var err error

	// 1. 验证工作节点名称
	if form.Name == "" {
		err = fmt.Errorf("工作节点名称不能为空")
		return err
	}

	// 2. 验证名称长度
	if len(form.Name) > 80 {
		err = fmt.Errorf("工作节点名称不能超过80个字符")
		return err
	}

	// 3. 验证描述长度
	if len(form.Description) > 1000 {
		err = fmt.Errorf("工作节点描述不能超过1000个字符")
		return err
	}

	return nil
}

// ToWorker 将表单转换为工作节点模型
func (form *WorkerCreateForm) ToWorker() *core.Worker {
	isActive := false
	if form.IsActive != nil {
		isActive = *form.IsActive
	} else {
		form.IsActive = &isActive
	}

	// 如果Metadata为空，则设置为{}空JSON对象
	metadata := form.Metadata
	if metadata == nil || len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	// 设置LastActive为当前时间，避免MySQL零值错误
	now := time.Now()

	// 处理ID：如果用户提供了ID则使用，否则生成新的UUID
	var id uuid.UUID
	if form.ID != "" {
		// 尝试解析用户提供的ID
		if parsedID, err := uuid.Parse(form.ID); err == nil {
			id = parsedID
		} else {
			// 用户提供的ID格式错误，生成新的UUID
			id = uuid.New()
		}
	} else {
		// 用户没有提供ID，生成新的UUID
		id = uuid.Nil
	}

	return &core.Worker{
		ID:          id,
		Name:        form.Name,
		Description: form.Description,
		IsActive:    form.IsActive,
		Metadata:    metadata,
		LastActive:  &now,
	}
}

// WorkerInfoForm 工作节点信息表单（用于更新）
type WorkerInfoForm struct {
	Name        string          `json:"name" form:"name"`
	Description string          `json:"description" form:"description"`
	IsActive    *bool           `json:"is_active" form:"is_active"`
	Metadata    json.RawMessage `json:"metadata" form:"metadata"`
}

// Validate 验证表单
func (form *WorkerInfoForm) Validate() error {
	var err error

	// 1. 验证名称长度
	if form.Name != "" && len(form.Name) > 128 {
		err = fmt.Errorf("工作节点名称不能超过128个字符")
		return err
	}

	// 2. 验证描述长度
	if len(form.Description) > 1000 {
		err = fmt.Errorf("工作节点描述不能超过1000个字符")
		return err
	}

	return nil
}

// UpdateWorker 根据表单更新工作节点信息
func (form *WorkerInfoForm) UpdateWorker(worker *core.Worker) {
	if form.Name != "" {
		worker.Name = form.Name
	}
	// 始终应用表单中的值，无论是否为空字符串
	// 这样可以支持将字段置空
	worker.Description = form.Description
	if form.IsActive != nil {
		worker.IsActive = form.IsActive
	}
	// 更新Metadata字段（如果提供了）
	if form.Metadata != nil {
		worker.Metadata = form.Metadata
	}
}
