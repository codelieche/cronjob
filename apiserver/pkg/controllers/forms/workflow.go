package forms

import (
	"fmt"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/google/uuid"
)

// WorkflowCreateForm 工作流创建表单
//
// 🔥 Steps 字段包含 core.WorkflowStep，已支持条件分支和并行执行：
//   - Condition: 条件表达式（可选）
//   - ParallelGroup: 并行组ID（可选）
//   - WaitStrategy: 等待策略（all/any/threshold:N）
//   - FailureStrategy: 失败策略（continue/abort）
//
// 前端可以直接在 steps 数组中的每个步骤对象中设置这些字段。
type WorkflowCreateForm struct {
	ID               string                 `json:"id" form:"id"`
	TeamID           string                 `json:"team_id" form:"team_id"`
	Project          string                 `json:"project" form:"project" example:"default"`
	Code             string                 `json:"code" form:"code" binding:"required" example:"cicd"`
	Name             string                 `json:"name" form:"name" binding:"required" example:"前端 CI/CD"`
	Description      string                 `json:"description" form:"description" example:"前端项目持续集成和部署"`
	Steps            []core.WorkflowStep    `json:"steps" form:"steps" binding:"required"`      // 🔥 包含条件分支和并行执行字段
	DefaultVariables map[string]interface{} `json:"default_variables" form:"default_variables"` // ⭐ 默认变量
	Metadata         *core.Metadata         `json:"metadata" form:"metadata"`
	IsActive         bool                   `json:"is_active" form:"is_active" example:"true"`
	Timeout          int                    `json:"timeout" form:"timeout" example:"3600"` // 工作流整体超时时间（秒），0表示默认24小时
}

// Validate 验证表单
func (form *WorkflowCreateForm) Validate() error {
	// 1. 验证名称
	if form.Name == "" {
		return fmt.Errorf("工作流名称不能为空")
	}
	if len(form.Name) > 256 {
		return fmt.Errorf("工作流名称不能超过256个字符")
	}

	// 2. 验证Code
	if form.Code == "" {
		return fmt.Errorf("工作流代码不能为空")
	}
	if len(form.Code) > 128 {
		return fmt.Errorf("工作流代码不能超过128个字符")
	}

	// 3. 验证步骤列表
	if len(form.Steps) == 0 {
		return fmt.Errorf("工作流步骤不能为空")
	}

	// 4. 验证每个步骤
	for i, step := range form.Steps {
		if step.Order <= 0 {
			return fmt.Errorf("第%d个步骤的序号必须大于0", i+1)
		}
		if step.Name == "" {
			return fmt.Errorf("第%d个步骤的名称不能为空", i+1)
		}
		if step.Category == "" {
			return fmt.Errorf("第%d个步骤的分类不能为空", i+1)
		}

		// 🔥 验证条件表达式（如果有）
		if step.Condition != "" {
			// 基本验证：长度限制
			if len(step.Condition) > 512 {
				return fmt.Errorf("第%d个步骤的条件表达式不能超过512个字符", i+1)
			}
		}

		// 🔥 验证并行组（如果有）
		if step.ParallelGroup != "" {
			// 验证并行组ID长度
			if len(step.ParallelGroup) > 128 {
				return fmt.Errorf("第%d个步骤的并行组ID不能超过128个字符", i+1)
			}
		}

		// 🔥 验证等待策略（如果有）
		if step.WaitStrategy != "" {
			validStrategies := []string{"all", "any"}
			isValid := false
			for _, strategy := range validStrategies {
				if step.WaitStrategy == strategy {
					isValid = true
					break
				}
			}
			// 检查是否是 threshold:N 格式
			if !isValid && len(step.WaitStrategy) > 10 && step.WaitStrategy[:10] == "threshold:" {
				isValid = true
			}
			if !isValid {
				return fmt.Errorf("第%d个步骤的等待策略无效，必须是 all、any 或 threshold:N", i+1)
			}
		}

		// 🔥 验证失败策略（如果有）
		if step.FailureStrategy != "" {
			if step.FailureStrategy != "continue" && step.FailureStrategy != "abort" {
				return fmt.Errorf("第%d个步骤的失败策略无效，必须是 continue 或 abort", i+1)
			}
		}
	}

	// 🔥 5. 验证并行组的一致性
	// 同一个并行组的所有步骤必须在同一个 Order
	parallelGroups := make(map[string]int) // parallelGroup -> order
	for _, step := range form.Steps {
		if step.ParallelGroup != "" {
			if existingOrder, exists := parallelGroups[step.ParallelGroup]; exists {
				if existingOrder != step.Order {
					return fmt.Errorf("并行组 %s 的步骤必须在同一个 Order（发现 Order %d 和 %d）",
						step.ParallelGroup, existingOrder, step.Order)
				}
			} else {
				parallelGroups[step.ParallelGroup] = step.Order
			}
		}
	}

	return nil
}

// ToWorkflow 转换为Workflow对象
func (form *WorkflowCreateForm) ToWorkflow() *core.Workflow {
	workflow := &core.Workflow{
		Project:     form.Project,
		Code:        form.Code,
		Name:        form.Name,
		Description: form.Description,
	}

	// 设置ID
	if form.ID != "" {
		if id, err := uuid.Parse(form.ID); err == nil {
			workflow.ID = id
		}
	}

	// 设置TeamID
	if form.TeamID != "" {
		if teamID, err := uuid.Parse(form.TeamID); err == nil {
			workflow.TeamID = &teamID
		}
	}

	// 设置Steps
	if err := workflow.SetSteps(form.Steps); err == nil {
		// Steps设置成功
	}

	// ⭐ 设置DefaultVariables
	if form.DefaultVariables != nil {
		if err := workflow.SetDefaultVariables(form.DefaultVariables); err == nil {
			// DefaultVariables设置成功
		}
	}

	// 设置Metadata
	if form.Metadata != nil {
		if err := workflow.SetMetadata(form.Metadata); err == nil {
			// Metadata设置成功
		}
	}

	// 设置IsActive
	workflow.IsActive = &form.IsActive

	// 🔥 设置Timeout
	workflow.Timeout = form.Timeout

	// 设置默认值
	if workflow.Project == "" {
		workflow.Project = "default"
	}

	return workflow
}

// WorkflowUpdateForm 工作流更新表单
//
// 🔥 Steps 字段包含 core.WorkflowStep，已支持条件分支和并行执行。
type WorkflowUpdateForm struct {
	Project          string                 `json:"project" form:"project"`
	Code             string                 `json:"code" form:"code"`
	Name             string                 `json:"name" form:"name"`
	Description      string                 `json:"description" form:"description"`
	Steps            []core.WorkflowStep    `json:"steps" form:"steps"`                         // 🔥 包含条件分支和并行执行字段
	DefaultVariables map[string]interface{} `json:"default_variables" form:"default_variables"` // ⭐ 默认变量
	Metadata         *core.Metadata         `json:"metadata" form:"metadata"`
	IsActive         *bool                  `json:"is_active" form:"is_active"`
	Timeout          int                    `json:"timeout" form:"timeout" example:"3600"` // 工作流整体超时时间（秒），0表示默认24小时
}

// Validate 验证表单
func (form *WorkflowUpdateForm) Validate() error {
	// 1. 验证名称长度
	if len(form.Name) > 256 {
		return fmt.Errorf("工作流名称不能超过256个字符")
	}

	// 2. 验证Code长度
	if len(form.Code) > 128 {
		return fmt.Errorf("工作流代码不能超过128个字符")
	}

	// 3. 如果有步骤列表，验证步骤
	if len(form.Steps) > 0 {
		for i, step := range form.Steps {
			if step.Order <= 0 {
				return fmt.Errorf("第%d个步骤的序号必须大于0", i+1)
			}
			if step.Name == "" {
				return fmt.Errorf("第%d个步骤的名称不能为空", i+1)
			}
			if step.Category == "" {
				return fmt.Errorf("第%d个步骤的分类不能为空", i+1)
			}

			// 🔥 验证条件表达式（如果有）
			if step.Condition != "" {
				if len(step.Condition) > 512 {
					return fmt.Errorf("第%d个步骤的条件表达式不能超过512个字符", i+1)
				}
			}

			// 🔥 验证并行组（如果有）
			if step.ParallelGroup != "" {
				if len(step.ParallelGroup) > 128 {
					return fmt.Errorf("第%d个步骤的并行组ID不能超过128个字符", i+1)
				}
			}

			// 🔥 验证等待策略（如果有）
			if step.WaitStrategy != "" {
				validStrategies := []string{"all", "any"}
				isValid := false
				for _, strategy := range validStrategies {
					if step.WaitStrategy == strategy {
						isValid = true
						break
					}
				}
				// 检查是否是 threshold:N 格式
				if !isValid && len(step.WaitStrategy) > 10 && step.WaitStrategy[:10] == "threshold:" {
					isValid = true
				}
				if !isValid {
					return fmt.Errorf("第%d个步骤的等待策略无效，必须是 all、any 或 threshold:N", i+1)
				}
			}

			// 🔥 验证失败策略（如果有）
			if step.FailureStrategy != "" {
				if step.FailureStrategy != "continue" && step.FailureStrategy != "abort" {
					return fmt.Errorf("第%d个步骤的失败策略无效，必须是 continue 或 abort", i+1)
				}
			}
		}

		// 🔥 验证并行组的一致性
		parallelGroups := make(map[string]int)
		for _, step := range form.Steps {
			if step.ParallelGroup != "" {
				if existingOrder, exists := parallelGroups[step.ParallelGroup]; exists {
					if existingOrder != step.Order {
						return fmt.Errorf("并行组 %s 的步骤必须在同一个 Order（发现 Order %d 和 %d）",
							step.ParallelGroup, existingOrder, step.Order)
					}
				} else {
					parallelGroups[step.ParallelGroup] = step.Order
				}
			}
		}
	}

	return nil
}

// ToWorkflow 转换为Workflow对象（用于更新）
func (form *WorkflowUpdateForm) ToWorkflow(id uuid.UUID) *core.Workflow {
	workflow := &core.Workflow{
		ID:          id,
		Project:     form.Project,
		Code:        form.Code,
		Name:        form.Name,
		Description: form.Description,
		IsActive:    form.IsActive,
		Timeout:     form.Timeout, // 🔥 设置Timeout
	}

	// 设置Steps（如果有）
	if len(form.Steps) > 0 {
		if err := workflow.SetSteps(form.Steps); err == nil {
			// Steps设置成功
		}
	}

	// ⭐ 设置DefaultVariables（如果有）
	if form.DefaultVariables != nil {
		if err := workflow.SetDefaultVariables(form.DefaultVariables); err == nil {
			// DefaultVariables设置成功
		}
	}

	// 设置Metadata（如果有）
	if form.Metadata != nil {
		if err := workflow.SetMetadata(form.Metadata); err == nil {
			// Metadata设置成功
		}
	}

	return workflow
}
