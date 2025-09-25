package forms

import (
	"fmt"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"github.com/google/uuid"
)

// CronJobCreateForm 定时任务创建表单
type CronJobCreateForm struct {
	ID          string                `json:"id" form:"id"`
	Project     string                `json:"project" form:"project"`
	Category    string                `json:"category" form:"category"`
	Name        string                `json:"name" form:"name" binding:"required"`
	Time        string                `json:"time" form:"time" binding:"required"`
	Command     string                `json:"command" form:"command" binding:"required"`
	Args        string                `json:"args" form:"args"`
	Description string                `json:"description" form:"description"`
	IsActive    bool                  `json:"is_active" form:"is_active"`
	SaveLog     bool                  `json:"save_log" form:"save_log"`
	Timeout     int                   `json:"timeout" form:"timeout"`
	Metadata    *core.CronJobMetadata `json:"metadata" form:"metadata"`
}

// Validate 验证表单
func (form *CronJobCreateForm) Validate() error {
	var err error

	// 1. 验证定时任务名称
	if form.Name == "" {
		err = fmt.Errorf("定时任务名称不能为空")
		return err
	}

	// 2. 验证名称长度
	if len(form.Name) > 128 {
		err = fmt.Errorf("定时任务名称不能超过128个字符")
		return err
	}

	// 3. 验证时间表达式
	if form.Time == "" {
		err = fmt.Errorf("定时任务时间表达式不能为空")
		return err
	}

	// 3.1 验证cron表达式格式是否正确
	if !tools.ValidateCronExpression(form.Time) {
		err = fmt.Errorf("定时任务时间表达式格式不正确")
		return err
	}

	// 4. 验证命令
	if form.Command == "" {
		err = fmt.Errorf("定时任务命令不能为空")
		return err
	}

	// 5. 验证字段长度
	if len(form.Project) > 128 {
		err = fmt.Errorf("项目名称不能超过128个字符")
		return err
	}

	if len(form.Category) > 128 {
		err = fmt.Errorf("分类编码不能超过128个字符")
		return err
	}

	if len(form.Time) > 100 {
		err = fmt.Errorf("时间表达式不能超过100个字符")
		return err
	}

	// 验证cron表达式格式是否正确
	if form.Time != "" && !tools.ValidateCronExpression(form.Time) {
		err = fmt.Errorf("定时任务时间表达式格式不正确")
		return err
	}

	if len(form.Command) > 512 {
		err = fmt.Errorf("命令不能超过512个字符")
		return err
	}

	if len(form.Args) > 512 {
		err = fmt.Errorf("参数不能超过512个字符")
		return err
	}

	if len(form.Description) > 512 {
		err = fmt.Errorf("描述不能超过512个字符")
		return err
	}

	return nil
}

// ToCronJob 将表单转换为定时任务模型
func (form *CronJobCreateForm) ToCronJob() *core.CronJob {
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
		// 用户没有提供ID，设置为空UUID，让存储层生成
		id = uuid.Nil
	}

	// 确保Project和Category不为空
	project := form.Project
	if project == "" {
		project = "default"
	}

	category := form.Category
	if category == "" {
		category = "default"
	}

	// 设置默认值
	isActive := form.IsActive
	saveLog := form.SaveLog

	// 设置时间字段为当前时间，避免MySQL零值错误
	// now := time.Now()

	cronJob := &core.CronJob{
		ID:           id,
		Project:      project,
		Category:     category,
		Name:         form.Name,
		Time:         form.Time,
		Command:      form.Command,
		Args:         form.Args,
		Description:  form.Description,
		LastPlan:     nil,
		LastDispatch: nil,
		IsActive:     &isActive,
		SaveLog:      &saveLog,
		Timeout:      form.Timeout,
	}

	// 处理元数据
	if form.Metadata != nil {
		if err := cronJob.SetMetadata(form.Metadata); err != nil {
			// 如果设置元数据失败，记录错误但不阻塞创建
			// 在实际应用中可能需要更严格的处理
			fmt.Printf("设置CronJob元数据失败: %v\n", err)
		}
	}

	return cronJob
}

// CronJobInfoForm 定时任务信息表单（用于更新）
type CronJobInfoForm struct {
	Project     string                `json:"project" form:"project"`
	Category    string                `json:"category" form:"category"`
	Name        string                `json:"name" form:"name"`
	Time        string                `json:"time" form:"time"`
	Command     string                `json:"command" form:"command"`
	Args        string                `json:"args" form:"args"`
	Description string                `json:"description" form:"description"`
	IsActive    bool                  `json:"is_active" form:"is_active"`
	SaveLog     bool                  `json:"save_log" form:"save_log"`
	Timeout     int                   `json:"timeout" form:"timeout"`
	Metadata    *core.CronJobMetadata `json:"metadata" form:"metadata"`
}

// Validate 验证表单
func (form *CronJobInfoForm) Validate() error {
	var err error

	// 验证字段长度
	if len(form.Project) > 128 {
		err = fmt.Errorf("项目名称不能超过128个字符")
		return err
	}

	if len(form.Category) > 128 {
		err = fmt.Errorf("分类编码不能超过128个字符")
		return err
	}

	if len(form.Name) > 128 {
		err = fmt.Errorf("定时任务名称不能超过128个字符")
		return err
	}

	if len(form.Time) > 100 {
		err = fmt.Errorf("时间表达式不能超过100个字符")
		return err
	}

	// 验证cron表达式格式是否正确
	if form.Time != "" && !tools.ValidateCronExpression(form.Time) {
		err = fmt.Errorf("定时任务时间表达式格式不正确")
		return err
	}

	if len(form.Command) > 512 {
		err = fmt.Errorf("命令不能超过512个字符")
		return err
	}

	if len(form.Args) > 512 {
		err = fmt.Errorf("参数不能超过512个字符")
		return err
	}

	if len(form.Description) > 512 {
		err = fmt.Errorf("描述不能超过512个字符")
		return err
	}

	return nil
}

// UpdateCronJob 根据表单更新定时任务信息
func (form *CronJobInfoForm) UpdateCronJob(CronJob *core.CronJob) {
	if form.Project != "" {
		CronJob.Project = form.Project
	}

	if form.Category != "" {
		CronJob.Category = form.Category
	} else if CronJob.Category == "" {
		// 如果Category为空，设置为default
		CronJob.Category = "default"
	}

	if form.Name != "" {
		CronJob.Name = form.Name
	}

	if form.Time != "" {
		CronJob.Time = form.Time
	}

	if form.Command != "" {
		CronJob.Command = form.Command
	}

	// 始终应用表单中的值，无论是否为空字符串
	// 这样可以支持将字段置空
	CronJob.Args = form.Args
	CronJob.Description = form.Description
	CronJob.IsActive = &form.IsActive
	CronJob.SaveLog = &form.SaveLog
	CronJob.Timeout = form.Timeout

	// 处理元数据更新
	if form.Metadata != nil {
		if err := CronJob.SetMetadata(form.Metadata); err != nil {
			// 如果设置元数据失败，记录错误但不阻塞更新
			fmt.Printf("更新CronJob元数据失败: %v\n", err)
		}
	}
}
