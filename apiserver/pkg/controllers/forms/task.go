package forms

import (
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
)

// TaskCreateForm 任务创建表单
type TaskCreateForm struct {
	ID           string    `json:"id" form:"id"`
	Project      string    `json:"project" form:"project"`
	Category     string    `json:"category" form:"category"`
	CronJob      string    `json:"cronjob" form:"cronjob"`
	Name         string    `json:"name" form:"name" binding:"required"`
	IsGroup      bool      `json:"is_group" form:"is_group"`
	TaskOrder    int       `json:"task_order" form:"task_order"`
	Previous     string    `json:"previous" form:"previous"`
	Next         string    `json:"next" form:"next"`
	Command      string    `json:"command" form:"command"`
	Args         string    `json:"args" form:"args"`
	Description  string    `json:"description" form:"description"`
	TimePlan     time.Time `json:"time_plan" form:"time_plan"`
	TimeoutAt    time.Time `json:"timeout_at" form:"timeout_at"`
	Status       string    `json:"status" form:"status"`
	SaveLog      bool      `json:"save_log" form:"save_log"`
	MaxRetry     int       `json:"max_retry" form:"max_retry"`
	WorkerID     string    `json:"worker_id" form:"worker_id"`
	WorkerName   string    `json:"worker_name" form:"worker_name"`
	IsStandalone bool      `json:"is_standalone" form:"is_standalone"`
	Timeout      int       `json:"timeout" form:"timeout"`
}

// Validate 验证表单
func (form *TaskCreateForm) Validate() error {
	var err error

	// 1. 验证任务名称
	if form.Name == "" {
		err = fmt.Errorf("任务名称不能为空")
		return err
	}

	// 2. 验证名称长度
	if len(form.Name) > 256 {
		err = fmt.Errorf("任务名称不能超过256个字符")
		return err
	}

	// 3. 验证描述长度
	if len(form.Description) > 512 {
		err = fmt.Errorf("任务描述不能超过512个字符")
		return err
	}

	// 4. 验证命令长度
	if len(form.Command) > 512 {
		err = fmt.Errorf("任务命令不能超过512个字符")
		return err
	}

	// 5. 验证参数长度
	if len(form.Args) > 512 {
		err = fmt.Errorf("任务参数不能超过512个字符")
		return err
	}

	// 6. 验证状态是否有效
	if form.Status != "" {
		validStatus := map[string]bool{
			core.TaskStatusPending:  true,
			core.TaskStatusRunning:  true,
			core.TaskStatusSuccess:  true,
			core.TaskStatusFailed:   true,
			core.TaskStatusError:    true,
			core.TaskStatusTimeout:  true,
			core.TaskStatusCanceled: true,
			core.TaskStatusRetrying: true,
		}

		if _, ok := validStatus[form.Status]; !ok {
			err = fmt.Errorf("无效的任务状态: %s", form.Status)
			return err
		}
	}

	// 7. 验证WorkerID格式
	if form.WorkerID != "" {
		if _, err := uuid.Parse(form.WorkerID); err != nil {
			err = fmt.Errorf("WorkerID格式无效")
			return err
		}
	}

	// 8. 验证CronJob格式
	if form.CronJob != "" {
		if _, err := uuid.Parse(form.CronJob); err != nil {
			err = fmt.Errorf("CronJobID格式无效")
			return err
		}
	}

	// 9. 验证Previous格式
	if form.Previous != "" {
		if _, err := uuid.Parse(form.Previous); err != nil {
			err = fmt.Errorf("Previous任务ID格式无效")
			return err
		}
	}

	// 10. 验证Next格式
	if form.Next != "" {
		if _, err := uuid.Parse(form.Next); err != nil {
			err = fmt.Errorf("Next任务ID格式无效")
			return err
		}
	}

	return nil
}

// ToTask 将表单转换为任务模型
func (form *TaskCreateForm) ToTask() *core.Task {
	// 处理ID
	var id uuid.UUID
	if form.ID != "" {
		if parsedID, err := uuid.Parse(form.ID); err == nil {
			id = parsedID
		} else {
			id = uuid.New()
		}
	} else {
		id = uuid.Nil
	}

	// 处理CronJob
	var cronJobID *uuid.UUID
	if form.CronJob != "" {
		if parsedID, err := uuid.Parse(form.CronJob); err == nil {
			cronJobID = &parsedID
		}
	}

	// 处理Previous
	var previousID *uuid.UUID
	if form.Previous != "" {
		if parsedID, err := uuid.Parse(form.Previous); err == nil {
			previousID = &parsedID
		}
	}

	// 处理Next
	var nextID *uuid.UUID
	if form.Next != "" {
		if parsedID, err := uuid.Parse(form.Next); err == nil {
			nextID = &parsedID
		}
	}

	// 处理WorkerID
	var workerID *uuid.UUID
	if form.WorkerID != "" {
		if parsedID, err := uuid.Parse(form.WorkerID); err == nil {
			workerID = &parsedID
		}
	}

	return &core.Task{
		ID:           id,
		Project:      form.Project,
		Category:     form.Category,
		CronJob:      cronJobID,
		Name:         form.Name,
		IsGroup:      form.IsGroup,
		TaskOrder:    form.TaskOrder,
		Previous:     previousID,
		Next:         nextID,
		Command:      form.Command,
		Args:         form.Args,
		Description:  form.Description,
		TimePlan:     form.TimePlan,
		TimeoutAt:    form.TimeoutAt,
		Status:       form.Status,
		SaveLog:      form.SaveLog,
		MaxRetry:     form.MaxRetry,
		WorkerID:     workerID,
		WorkerName:   form.WorkerName,
		IsStandalone: form.IsStandalone,
		Timeout:      form.Timeout,
	}
}

// TaskInfoForm 任务信息表单（用于更新）
type TaskInfoForm struct {
	Project      string    `json:"project" form:"project"`
	Category     string    `json:"category" form:"category"`
	CronJob      string    `json:"cronjob" form:"cronjob"`
	Name         string    `json:"name" form:"name"`
	IsGroup      bool      `json:"is_group" form:"is_group"`
	TaskOrder    int       `json:"task_order" form:"task_order"`
	Timeout      int       `json:"timeout" form:"timeout"`
	Previous     string    `json:"previous" form:"previous"`
	Next         string    `json:"next" form:"next"`
	Command      string    `json:"command" form:"command"`
	Args         string    `json:"args" form:"args"`
	Description  string    `json:"description" form:"description"`
	TimePlan     time.Time `json:"time_plan" form:"time_plan"`
	TimeoutAt    time.Time `json:"timeout_at" form:"timeout_at"`
	Status       string    `json:"status" form:"status"`
	Output       string    `json:"output" form:"output"`
	SaveLog      bool      `json:"save_log" form:"save_log"`
	RetryCount   int       `json:"retry_count" form:"retry_count"`
	MaxRetry     int       `json:"max_retry" form:"max_retry"`
	WorkerID     string    `json:"worker_id" form:"worker_id"`
	WorkerName   string    `json:"worker_name" form:"worker_name"`
	IsStandalone bool      `json:"is_standalone" form:"is_standalone"`
}

// Validate 验证表单
func (form *TaskInfoForm) Validate() error {
	var err error

	// 验证名称长度
	if form.Name != "" && len(form.Name) > 256 {
		err = fmt.Errorf("任务名称不能超过256个字符")
		return err
	}

	// 验证描述长度
	if form.Description != "" && len(form.Description) > 512 {
		err = fmt.Errorf("任务描述不能超过512个字符")
		return err
	}

	// 验证命令长度
	if form.Command != "" && len(form.Command) > 512 {
		err = fmt.Errorf("任务命令不能超过512个字符")
		return err
	}

	// 验证参数长度
	if form.Args != "" && len(form.Args) > 512 {
		err = fmt.Errorf("任务参数不能超过512个字符")
		return err
	}

	// 验证状态是否有效
	if form.Status != "" {
		validStatus := map[string]bool{
			core.TaskStatusPending:  true,
			core.TaskStatusRunning:  true,
			core.TaskStatusSuccess:  true,
			core.TaskStatusFailed:   true,
			core.TaskStatusError:    true,
			core.TaskStatusTimeout:  true,
			core.TaskStatusCanceled: true,
			core.TaskStatusRetrying: true,
		}

		if _, ok := validStatus[form.Status]; !ok {
			err = fmt.Errorf("无效的任务状态: %s", form.Status)
			return err
		}
	}

	// 验证WorkerID格式
	if form.WorkerID != "" {
		if _, err := uuid.Parse(form.WorkerID); err != nil {
			err = fmt.Errorf("WorkerID格式无效")
			return err
		}
	}

	// 验证CronJob格式
	if form.CronJob != "" {
		if _, err := uuid.Parse(form.CronJob); err != nil {
			err = fmt.Errorf("CronJobID格式无效")
			return err
		}
	}

	// 验证Previous格式
	if form.Previous != "" {
		if _, err := uuid.Parse(form.Previous); err != nil {
			err = fmt.Errorf("Previous任务ID格式无效")
			return err
		}
	}

	// 验证Next格式
	if form.Next != "" {
		if _, err := uuid.Parse(form.Next); err != nil {
			err = fmt.Errorf("Next任务ID格式无效")
			return err
		}
	}

	return nil
}
