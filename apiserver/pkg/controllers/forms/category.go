package forms

import (
	"fmt"
	"regexp"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
)

// CategoryCreateForm 分类创建表单
type CategoryCreateForm struct {
	Code        string `json:"code" form:"code" binding:"required" example:"backup"`
	Name        string `json:"name" form:"name" example:"数据备份分类"`
	Setup       string `json:"setup" form:"setup" example:"mkdir -p /tmp/backup"`
	Teardown    string `json:"teardown" form:"teardown" example:"rm -rf /tmp/backup"`
	Check       string `json:"check" form:"check" example:"test -d /backup"`
	Description string `json:"description" form:"description" example:"用于数据库备份相关的任务分类"`
}

// Validate 验证表单
func (form *CategoryCreateForm) Validate() error {
	var err error

	// 1. 验证分类编码
	if form.Code == "" {
		err = fmt.Errorf("分类编码不能为空")
		return err
	} else {
		// 验证编码不可是纯数字
		matched, _ := regexp.MatchString("^[0-9].*", form.Code)
		if matched {
			err = fmt.Errorf("分类编码不能是纯数字")
			return err
		}
	}

	// 2. 验证编码长度
	if len(form.Code) > 128 {
		err = fmt.Errorf("分类编码不能超过128个字符")
		return err
	}

	// 3. 验证名称长度
	if len(form.Name) > 128 {
		err = fmt.Errorf("分类名称不能超过128个字符")
		return err
	}

	// 4. 验证脚本长度
	if len(form.Setup) > 512 {
		err = fmt.Errorf("初始化脚本不能超过512个字符")
		return err
	}

	if len(form.Teardown) > 512 {
		err = fmt.Errorf("销毁脚本不能超过512个字符")
		return err
	}

	if len(form.Check) > 512 {
		err = fmt.Errorf("检查脚本不能超过512个字符")
		return err
	}

	// 5. 验证描述长度
	if len(form.Description) > 1000 {
		err = fmt.Errorf("分类描述不能超过1000个字符")
		return err
	}

	return nil
}

// ToCategory 将表单转换为分类模型
func (form *CategoryCreateForm) ToCategory() *core.Category {
	return &core.Category{
		Code:        form.Code,
		Name:        form.Name,
		Setup:       form.Setup,
		Teardown:    form.Teardown,
		Check:       form.Check,
		Description: form.Description,
	}
}

// CategoryInfoForm 分类信息表单（用于更新）
type CategoryInfoForm struct {
	Code        string `json:"code" form:"code"`
	Name        string `json:"name" form:"name"`
	Setup       string `json:"setup" form:"setup"`
	Teardown    string `json:"teardown" form:"teardown"`
	Check       string `json:"check" form:"check"`
	Description string `json:"description" form:"description"`
}

// Validate 验证表单
func (form *CategoryInfoForm) Validate() error {
	var err error

	// 1. 验证编码长度
	if form.Code != "" {
		if len(form.Code) > 128 {
			err = fmt.Errorf("分类编码不能超过128个字符")
			return err
		} else {
			// 验证编码不可是纯数字
			matched, _ := regexp.MatchString("^[0-9].*", form.Code)
			if matched {
				err = fmt.Errorf("分类编码不能是纯数字")
				return err
			}
		}
	}

	// 2. 验证名称长度
	if form.Name != "" && len(form.Name) > 128 {
		err = fmt.Errorf("分类名称不能超过128个字符")
		return err
	}

	// 3. 验证脚本长度
	if form.Setup != "" && len(form.Setup) > 512 {
		err = fmt.Errorf("初始化脚本不能超过512个字符")
		return err
	}

	if form.Teardown != "" && len(form.Teardown) > 512 {
		err = fmt.Errorf("销毁脚本不能超过512个字符")
		return err
	}

	if form.Check != "" && len(form.Check) > 512 {
		err = fmt.Errorf("检查脚本不能超过512个字符")
		return err
	}

	// 4. 验证描述长度
	if form.Description != "" && len(form.Description) > 1000 {
		err = fmt.Errorf("分类描述不能超过1000个字符")
		return err
	}

	return nil
}

// UpdateCategory 更新分类信息
func (form *CategoryInfoForm) UpdateCategory(category *core.Category) {
	// 始终应用表单中的值，无论是否为空字符串
	// 这样可以支持将字段置空
	category.Code = form.Code
	category.Name = form.Name
	category.Setup = form.Setup
	category.Teardown = form.Teardown
	category.Check = form.Check
	category.Description = form.Description
}
