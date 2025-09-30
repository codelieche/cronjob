package forms

import (
	"fmt"
	"strings"

	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
)

// validateStorageType 验证存储类型的通用函数
func validateStorageType(storage string) error {
	validStorages := []string{core.TaskLogStorageDB, core.TaskLogStorageFile, core.TaskLogStorageS3}
	for _, validStorage := range validStorages {
		if storage == validStorage {
			return nil
		}
	}
	return fmt.Errorf("存储类型无效，支持的类型：%s", strings.Join(validStorages, ", "))
}

// TaskLogCreateForm 任务日志创建表单
type TaskLogCreateForm struct {
	TaskID  string `json:"task_id" form:"task_id" binding:"required"` // 任务ID
	Storage string `json:"storage" form:"storage"`                    // 存储类型
	Content string `json:"content" form:"content"`                    // 日志内容（仅db存储时使用）
}

// Validate 验证表单
func (form *TaskLogCreateForm) Validate() error {
	var err error

	// 1. 验证任务ID
	if form.TaskID == "" {
		err = fmt.Errorf("任务ID不能为空")
		return err
	}

	// 验证任务ID格式
	if _, err := uuid.Parse(form.TaskID); err != nil {
		err = fmt.Errorf("任务ID格式无效")
		return err
	}

	// 2. 验证存储类型
	if form.Storage == "" {
		form.Storage = config.Web.LogStorage // 默认使用数据库存储
	} else if err := validateStorageType(form.Storage); err != nil {
		return err
	}

	// 3. 验证内容（仅db存储时需要）
	if form.Storage == core.TaskLogStorageDB && form.Content == "" {
		err = fmt.Errorf("数据库存储时日志内容不能为空")
		return err
	}

	return nil
}

// ToTaskLog 将表单转换为任务日志模型
func (form *TaskLogCreateForm) ToTaskLog() *core.TaskLog {
	taskID, _ := uuid.Parse(form.TaskID) // 已在Validate中验证过

	// 根据存储类型设置初始值
	content := form.Content

	return &core.TaskLog{
		TaskID:  taskID,
		Storage: form.Storage,
		Path:    "", // 路径由系统自动生成
		Content: content,
		Size:    int64(len(content)),
	}
}

// TaskLogUpdateForm 任务日志更新表单
type TaskLogUpdateForm struct {
	Storage string `json:"storage" form:"storage"` // 存储类型
	Content string `json:"content" form:"content"` // 日志内容（仅db存储时使用）
}

// Validate 验证表单
func (form *TaskLogUpdateForm) Validate() error {
	var err error

	// 1. 验证存储类型（如果提供了）
	if form.Storage != "" {
		if err := validateStorageType(form.Storage); err != nil {
			return err
		}
	}

	// 2. 验证内容（仅db存储时需要）
	if form.Storage == core.TaskLogStorageDB && form.Content == "" {
		err = fmt.Errorf("数据库存储时日志内容不能为空")
		return err
	}

	return nil
}

// UpdateTaskLog 根据表单更新任务日志信息
func (form *TaskLogUpdateForm) UpdateTaskLog(taskLog *core.TaskLog) {
	if form.Storage != "" {
		taskLog.Storage = form.Storage
		// 如果存储类型改变，需要重新生成路径
		if form.Storage != core.TaskLogStorageDB {
			taskLog.Content = "" // 非数据库存储时，清空content
		}
	}
	if form.Content != "" && form.Storage == core.TaskLogStorageDB {
		taskLog.Content = form.Content
		taskLog.Size = int64(len(form.Content))
	}
}

// TaskLogContentForm 任务日志内容表单
type TaskLogContentForm struct {
	Content string `json:"content" form:"content" binding:"required"` // 日志内容
}

// Validate 验证表单
func (form *TaskLogContentForm) Validate() error {
	if form.Content == "" {
		return fmt.Errorf("日志内容不能为空")
	}
	return nil
}

// TaskLogAppendForm 任务日志追加表单
type TaskLogAppendForm struct {
	Content string `json:"content" form:"content" binding:"required"` // 要追加的日志内容
}

// Validate 验证表单
func (form *TaskLogAppendForm) Validate() error {
	if form.Content == "" {
		return fmt.Errorf("要追加的日志内容不能为空")
	}
	return nil
}
