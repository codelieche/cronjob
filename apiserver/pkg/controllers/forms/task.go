package forms

import (
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
)

// TaskCreateForm 任务创建表单
type TaskCreateForm struct {
	ID           string         `json:"id" form:"id" example:""`
	TeamID       string         `json:"team_id" form:"team_id" example:"123e4567-e89b-12d3-a456-426614174000"`
	Project      string         `json:"project" form:"project" example:"web-backend"`
	Category     string         `json:"category" form:"category" example:"backup"`
	CronJob      string         `json:"cronjob" form:"cronjob" example:"987fcdeb-51a2-43d7-8f6e-123456789abc"`
	Workflow     string         `json:"workflow" form:"workflow" example:"123e4567-e89b-12d3-a456-426614174000"`
	Name         string         `json:"name" form:"name" binding:"required" example:"数据库备份任务执行"`
	IsGroup      bool           `json:"is_group" form:"is_group" example:"false"`
	TaskOrder    int            `json:"task_order" form:"task_order" example:"1"`
	Previous     string         `json:"previous" form:"previous" example:""`
	Next         string         `json:"next" form:"next" example:""`
	Command      string         `json:"command" form:"command" example:"backup_database.sh"`
	Args         string         `json:"args" form:"args" example:"--full --compress"`
	Description  string         `json:"description" form:"description" example:"执行数据库全量备份"`
	TimePlan     time.Time      `json:"time_plan" form:"time_plan" example:"2025-09-30T02:00:00Z"`
	TimeoutAt    time.Time      `json:"timeout_at" form:"timeout_at" example:"2025-09-30T02:05:00Z"`
	Status       string         `json:"status" form:"status" example:"pending"`
	SaveLog      bool           `json:"save_log" form:"save_log" example:"true"`
	MaxRetry     int            `json:"max_retry" form:"max_retry" example:"3"`
	WorkerID     string         `json:"worker_id" form:"worker_id" example:"456e7890-f12a-34b5-c678-9012345678de"`
	WorkerName   string         `json:"worker_name" form:"worker_name" example:"worker-node-01"`
	IsStandalone bool           `json:"is_standalone" form:"is_standalone" example:"false"`
	Timeout      int            `json:"timeout" form:"timeout" example:"300"`
	Metadata     *core.Metadata `json:"metadata" form:"metadata"`
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
	// Args 字段现在是 TEXT 类型，最大支持 64KB
	// 为了防止恶意提交超大数据，设置一个合理的上限
	if len(form.Args) > 65535 {
		err = fmt.Errorf("任务参数不能超过64KB (65535字节)")
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

	// 9. 验证Workflow格式
	if form.Workflow != "" {
		if _, err := uuid.Parse(form.Workflow); err != nil {
			err = fmt.Errorf("WorkflowID格式无效")
			return err
		}
	}

	// 10. 验证Previous格式
	if form.Previous != "" {
		if _, err := uuid.Parse(form.Previous); err != nil {
			err = fmt.Errorf("Previous任务ID格式无效")
			return err
		}
	}

	// 11. 验证Next格式
	if form.Next != "" {
		if _, err := uuid.Parse(form.Next); err != nil {
			err = fmt.Errorf("Next任务ID格式无效")
			return err
		}
	}

	// 12. 验证TeamID格式
	if form.TeamID != "" {
		if _, err := uuid.Parse(form.TeamID); err != nil {
			err = fmt.Errorf("TeamID格式无效")
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

	// 处理Workflow
	var workflowID *uuid.UUID
	if form.Workflow != "" {
		if parsedID, err := uuid.Parse(form.Workflow); err == nil {
			workflowID = &parsedID
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

	// 处理TeamID
	var teamID *uuid.UUID
	if form.TeamID != "" {
		if parsedID, err := uuid.Parse(form.TeamID); err == nil {
			teamID = &parsedID
		}
	}

	// 处理WorkerID
	var workerID *uuid.UUID
	if form.WorkerID != "" {
		if parsedID, err := uuid.Parse(form.WorkerID); err == nil {
			workerID = &parsedID
		}
	}

	task := &core.Task{
		ID:           id,
		TeamID:       teamID,
		Project:      form.Project,
		Category:     form.Category,
		CronJob:      cronJobID,
		Workflow:     workflowID,
		Name:         form.Name,
		IsGroup:      &form.IsGroup,
		TaskOrder:    form.TaskOrder,
		Previous:     previousID,
		Next:         nextID,
		Command:      form.Command,
		Args:         form.Args,
		Description:  form.Description,
		TimePlan:     form.TimePlan,
		TimeoutAt:    form.TimeoutAt,
		Status:       form.Status,
		SaveLog:      &form.SaveLog,
		MaxRetry:     form.MaxRetry,
		WorkerID:     workerID,
		WorkerName:   form.WorkerName,
		IsStandalone: &form.IsStandalone,
		Timeout:      form.Timeout,
	}

	// 处理元数据
	if form.Metadata != nil {
		if err := task.SetMetadata(form.Metadata); err != nil {
			// 如果设置元数据失败，记录错误但不阻塞创建
			fmt.Printf("设置Task元数据失败: %v\n", err)
		}
	}

	return task
}

// TaskInfoForm 任务信息表单（用于更新）
type TaskInfoForm struct {
	TeamID       string         `json:"team_id" form:"team_id"`
	Project      string         `json:"project" form:"project"`
	Category     string         `json:"category" form:"category"`
	CronJob      string         `json:"cronjob" form:"cronjob"`
	Workflow     string         `json:"workflow" form:"workflow"`
	Name         string         `json:"name" form:"name"`
	IsGroup      bool           `json:"is_group" form:"is_group"`
	TaskOrder    int            `json:"task_order" form:"task_order"`
	Timeout      int            `json:"timeout" form:"timeout"`
	Previous     string         `json:"previous" form:"previous"`
	Next         string         `json:"next" form:"next"`
	Command      string         `json:"command" form:"command"`
	Args         string         `json:"args" form:"args"`
	Description  string         `json:"description" form:"description"`
	TimePlan     time.Time      `json:"time_plan" form:"time_plan"`
	TimeoutAt    time.Time      `json:"timeout_at" form:"timeout_at"`
	Status       string         `json:"status" form:"status"`
	Output       string         `json:"output" form:"output"`
	SaveLog      bool           `json:"save_log" form:"save_log"`
	RetryCount   int            `json:"retry_count" form:"retry_count"`
	MaxRetry     int            `json:"max_retry" form:"max_retry"`
	WorkerID     string         `json:"worker_id" form:"worker_id"`
	WorkerName   string         `json:"worker_name" form:"worker_name"`
	IsStandalone bool           `json:"is_standalone" form:"is_standalone"`
	Metadata     *core.Metadata `json:"metadata" form:"metadata"`
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
	// Args 字段现在是 TEXT 类型，最大支持 64KB
	// 为了防止恶意提交超大数据，设置一个合理的上限
	if form.Args != "" && len(form.Args) > 65535 {
		err = fmt.Errorf("任务参数不能超过64KB (65535字节)")
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
			core.TaskStatusStopped:  true, // 🔥 新增stopped状态
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

	// 验证Workflow格式
	if form.Workflow != "" {
		if _, err := uuid.Parse(form.Workflow); err != nil {
			err = fmt.Errorf("WorkflowID格式无效")
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

	// 验证TeamID格式
	if form.TeamID != "" {
		if _, err := uuid.Parse(form.TeamID); err != nil {
			err = fmt.Errorf("TeamID格式无效")
			return err
		}
	}

	return nil
}

// UpdateTask 根据表单更新任务信息
func (form *TaskInfoForm) UpdateTask(task *core.Task) error {
	// 处理TeamID更新
	if form.TeamID != "" {
		if parsedTeamID, err := uuid.Parse(form.TeamID); err == nil {
			task.TeamID = &parsedTeamID
		}
	}

	// 更新基本字段（如果提供了非空值）
	if form.Project != "" {
		task.Project = form.Project
	}
	if form.Category != "" {
		task.Category = form.Category
	}
	if form.Name != "" {
		task.Name = form.Name
	}
	if form.Command != "" {
		task.Command = form.Command
	}

	// 处理可能为空的字段
	task.Args = form.Args
	task.Description = form.Description
	task.Output = form.Output
	task.IsGroup = &form.IsGroup
	task.TaskOrder = form.TaskOrder
	task.SaveLog = &form.SaveLog
	task.RetryCount = form.RetryCount
	task.MaxRetry = form.MaxRetry
	task.WorkerName = form.WorkerName
	task.IsStandalone = &form.IsStandalone
	task.Timeout = form.Timeout

	// 处理时间字段（如果不是零值）
	if !form.TimePlan.IsZero() {
		task.TimePlan = form.TimePlan
	}
	if !form.TimeoutAt.IsZero() {
		task.TimeoutAt = form.TimeoutAt
	}

	// 处理状态更新
	if form.Status != "" {
		task.Status = form.Status
	}

	// 处理UUID字段
	if form.CronJob != "" {
		if parsedID, err := uuid.Parse(form.CronJob); err == nil {
			task.CronJob = &parsedID
		}
	}
	if form.Workflow != "" {
		if parsedID, err := uuid.Parse(form.Workflow); err == nil {
			task.Workflow = &parsedID
		}
	}
	if form.Previous != "" {
		if parsedID, err := uuid.Parse(form.Previous); err == nil {
			task.Previous = &parsedID
		}
	}
	if form.Next != "" {
		if parsedID, err := uuid.Parse(form.Next); err == nil {
			task.Next = &parsedID
		}
	}
	if form.WorkerID != "" {
		if parsedID, err := uuid.Parse(form.WorkerID); err == nil {
			task.WorkerID = &parsedID
		}
	}

	// 处理元数据更新
	if form.Metadata != nil {
		if err := task.SetMetadata(form.Metadata); err != nil {
			return fmt.Errorf("更新Task元数据失败: %v", err)
		}
	}

	return nil
}

// StopTaskRequest 停止任务请求参数
type StopTaskRequest struct {
	Force bool `json:"force" form:"force"` // false=优雅停止(SIGTERM), true=强制终止(SIGKILL)
}

// Validate 验证表单
func (form *StopTaskRequest) Validate() error {
	// force参数是布尔值，不需要特殊验证
	return nil
}
