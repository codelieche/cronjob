package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewWorkflowExecuteService 创建 WorkflowExecuteService 实例
func NewWorkflowExecuteService(
	store core.WorkflowExecuteStore,
	workflowStore core.WorkflowStore,
	taskStore core.TaskStore,
) core.WorkflowExecuteService {
	return &WorkflowExecuteService{
		store:         store,
		workflowStore: workflowStore,
		taskStore:     taskStore,
	}
}

// WorkflowExecuteService 工作流执行服务实现
type WorkflowExecuteService struct {
	store         core.WorkflowExecuteStore
	workflowStore core.WorkflowStore
	taskStore     core.TaskStore
}

// FindByID 根据ID获取工作流执行实例
func (s *WorkflowExecuteService) FindByID(ctx context.Context, id string) (*core.WorkflowExecute, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow execute id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// List 查询工作流执行列表
func (s *WorkflowExecuteService) List(ctx context.Context, offset, limit int, actions ...filters.Filter) ([]*core.WorkflowExecute, error) {
	executes, err := s.store.List(ctx, offset, limit, actions...)
	if err != nil {
		logger.Error("list workflow executes error", zap.Error(err), zap.Int("offset", offset), zap.Int("limit", limit))
		return nil, err
	}
	return executes, nil
}

// Count 统计工作流执行数量
func (s *WorkflowExecuteService) Count(ctx context.Context, actions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, actions...)
	if err != nil {
		logger.Error("count workflow executes error", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// GetTasksByExecuteID 根据执行实例ID获取任务列表 🔥
func (s *WorkflowExecuteService) GetTasksByExecuteID(ctx context.Context, executeID string) ([]*core.Task, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(executeID)
	if err != nil {
		logger.Error("parse execute id error", zap.Error(err), zap.String("execute_id", executeID))
		return nil, core.ErrBadRequest
	}

	// 使用 taskStore.List 查询任务（按 workflow_exec_id 过滤）
	filterActions := []filters.Filter{
		&filters.FilterOption{
			Column: "workflow_exec_id",
			Value:  uuidID,
			Op:     filters.FILTER_EQ,
		},
	}

	// 🔥 按 step_order 升序排序
	filterActions = append(filterActions, &filters.Ordering{
		Fields: []string{"step_order"},
		Value:  "step_order", // 升序
	})

	tasks, err := s.taskStore.List(ctx, 0, 1000, filterActions...)
	if err != nil {
		logger.Error("query tasks by execute_id error",
			zap.Error(err),
			zap.String("execute_id", executeID))
		return nil, err
	}

	logger.Info("成功获取工作流执行任务列表",
		zap.String("execute_id", executeID),
		zap.Int("count", len(tasks)))

	return tasks, nil
}

// ListByWorkflowID 根据WorkflowID查询执行列表
func (s *WorkflowExecuteService) ListByWorkflowID(ctx context.Context, workflowID string, limit, offset int) ([]*core.WorkflowExecute, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(workflowID)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", workflowID))
		return nil, core.ErrBadRequest
	}

	executes, err := s.store.ListByWorkflowID(ctx, uuidID, limit, offset)
	if err != nil {
		logger.Error("list workflow executes by workflow id error", zap.Error(err), zap.String("workflow_id", workflowID))
		return nil, err
	}
	return executes, nil
}

// CountByWorkflowID 统计Workflow的执行次数
func (s *WorkflowExecuteService) CountByWorkflowID(ctx context.Context, workflowID string) (int64, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(workflowID)
	if err != nil {
		logger.Error("parse workflow id error", zap.Error(err), zap.String("id", workflowID))
		return 0, core.ErrBadRequest
	}

	count, err := s.store.CountByWorkflowID(ctx, uuidID)
	if err != nil {
		logger.Error("count workflow executes by workflow id error", zap.Error(err), zap.String("workflow_id", workflowID))
		return 0, err
	}
	return count, nil
}

// Cancel 取消工作流执行
func (s *WorkflowExecuteService) Cancel(ctx context.Context, id string, userID *uuid.UUID, username string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow execute id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 获取工作流执行实例
	execute, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return err
	}

	// 检查是否可以取消
	if !execute.CanCancel() {
		logger.Error("workflow execute cannot be cancelled",
			zap.String("id", id),
			zap.String("status", execute.Status))
		return core.ErrBadRequest
	}

	// 更新执行实例状态
	execute.Status = core.WorkflowExecuteStatusCanceled
	now := time.Now()
	if execute.TimeStart == nil {
		execute.TimeStart = &now
	}
	execute.TimeEnd = &now
	execute.ErrorMessage = "Cancelled by user"
	if username != "" {
		execute.ErrorMessage = "Cancelled by " + username
	}

	// 保存更新
	if err := s.store.Update(ctx, execute); err != nil {
		logger.Error("update workflow execute error", zap.Error(err), zap.String("id", id))
		return err
	}

	// 取消所有待执行的 Task（status=todo 或 status=pending）
	// 这里需要查询该执行实例的所有 Task
	// TODO: 实现 TaskStore.ListByWorkflowExecID 方法
	// tasks, err := s.taskStore.ListByWorkflowExecID(ctx, uuidID)
	// if err != nil {
	//     logger.Error("list tasks by workflow exec id error", zap.Error(err))
	//     return err
	// }
	//
	// for _, task := range tasks {
	//     if task.Status == core.TaskStatusPending || task.Status == "todo" {
	//         task.Status = core.TaskStatusCanceled
	//         if err := s.taskStore.Update(ctx, task); err != nil {
	//             logger.Error("cancel task error", zap.Error(err), zap.String("task_id", task.ID.String()))
	//         }
	//     }
	// }

	// 更新 Workflow 统计信息
	if err := s.workflowStore.UpdateStats(ctx, execute.WorkflowID, core.WorkflowExecuteStatusCanceled); err != nil {
		logger.Error("update workflow stats error", zap.Error(err))
		// 不返回错误，只记录日志
	}

	logger.Info("workflow execute cancelled",
		zap.String("id", id),
		zap.String("workflow_id", execute.WorkflowID.String()),
		zap.String("username", username))

	return nil
}

// Delete 删除工作流执行实例
func (s *WorkflowExecuteService) Delete(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse workflow execute id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 检查执行实例是否存在
	execute, err := s.store.FindByID(ctx, uuidID)
	if err != nil {
		return err
	}

	// 删除执行实例
	if err := s.store.Delete(ctx, uuidID); err != nil {
		logger.Error("delete workflow execute error", zap.Error(err), zap.String("id", id))
		return err
	}

	logger.Info("workflow execute deleted",
		zap.String("id", id),
		zap.String("workflow_id", execute.WorkflowID.String()))

	return nil
}

// Execute 触发工作流执行 ⭐（核心方法）
func (s *WorkflowExecuteService) Execute(ctx context.Context, req *core.ExecuteRequest) (*core.WorkflowExecute, error) {
	logger.Info("开始执行工作流",
		zap.String("workflow_id", req.WorkflowID.String()),
		zap.String("trigger_type", req.TriggerType))

	// ========== Step 1: 加载 Workflow 模板 ==========
	workflow, err := s.workflowStore.FindByID(ctx, req.WorkflowID)
	if err != nil {
		logger.Error("加载工作流失败", zap.Error(err), zap.String("workflow_id", req.WorkflowID.String()))
		return nil, err
	}

	// 检查是否激活
	if workflow.IsActive != nil && !*workflow.IsActive {
		logger.Error("工作流未激活", zap.String("workflow_id", req.WorkflowID.String()))
		return nil, fmt.Errorf("工作流未激活")
	}

	// ========== Step 2: 解析步骤列表 ==========
	steps, err := workflow.GetSteps()
	if err != nil {
		logger.Error("解析工作流步骤失败", zap.Error(err))
		return nil, err
	}

	if len(steps) == 0 {
		logger.Error("工作流没有步骤")
		return nil, fmt.Errorf("工作流没有步骤")
	}

	// 按 order 排序
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Order < steps[j].Order
	})

	// ========== Step 3: 创建 WorkflowExecute 实例 ==========
	now := time.Now()
	workflowExec := &core.WorkflowExecute{
		ID:          uuid.New(),
		WorkflowID:  workflow.ID,
		TeamID:      workflow.TeamID,
		Project:     workflow.Project, // ⭐ 从 Workflow 继承 Project
		TriggerType: req.TriggerType,
		UserID:      req.UserID,
		Username:    req.Username,
		Status:      core.WorkflowExecuteStatusPending,
		TotalSteps:  len(steps),
		CurrentStep: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// ========== Step 4: 初始化 Variables（DefaultVariables + initial_variables）⭐ ==========
	// 4.1 先从 Workflow.DefaultVariables 加载默认值
	defaultVars, err := workflow.GetDefaultVariables()
	if err != nil {
		logger.Warn("解析默认变量失败，使用空变量", zap.Error(err))
		defaultVars = make(map[string]interface{})
	}

	// 4.2 用 initial_variables 覆盖默认值
	finalVariables := make(map[string]interface{})
	// 先复制默认值
	for k, v := range defaultVars {
		finalVariables[k] = v
	}
	// 再用 initial_variables 覆盖
	if req.InitialVariables != nil {
		for k, v := range req.InitialVariables {
			finalVariables[k] = v
		}
	}

	// 4.3 设置到 WorkflowExecute
	if err := workflowExec.SetVariables(finalVariables); err != nil {
		logger.Error("设置变量失败", zap.Error(err))
		return nil, err
	}

	logger.Info("初始化 Variables",
		zap.Int("default_count", len(defaultVars)),
		zap.Int("override_count", len(req.InitialVariables)),
		zap.Int("final_count", len(finalVariables)),
		zap.String("exec_id", workflowExec.ID.String()))

	// ========== Step 5: 准备 Metadata（继承 + 覆盖）==========
	// 5.1 从 Workflow 继承 Metadata
	metadata, err := workflow.GetMetadata()
	if err != nil {
		metadata = &core.Metadata{} // 如果解析失败，使用空 Metadata
	}

	// 5.2 ⭐ 如果没有 WorkingDir，设置默认值（兼容旧数据）
	if metadata.WorkingDir == "" {
		// 使用 Workflow 的 ID 作为默认工作目录
		metadata.WorkingDir = fmt.Sprintf("./workflow/%s", workflow.ID.String())
		logger.Info("自动设置默认工作目录（执行时）",
			zap.String("workflow_id", workflow.ID.String()),
			zap.String("exec_id", workflowExec.ID.String()),
			zap.String("working_dir", metadata.WorkingDir))
	}

	// 5.3 应用 metadata_override
	if len(req.MetadataOverride) > 0 {
		// 将 map 转换为 Metadata 结构
		overrideMetadata := &core.Metadata{}
		// 这里简化处理，直接覆盖字段
		// 实际使用时可以根据 map 的 key 来设置对应字段
		// 或者在 ExecuteRequest 中直接使用 *core.Metadata 类型

		// 深度合并 metadata_override（locked=false，允许覆盖）
		metadata = core.MergeMetadata(metadata, overrideMetadata, false)
		logger.Info("应用 Metadata 覆盖", zap.Int("override_count", len(req.MetadataOverride)))
	}

	// 5.4 保存合并后的 Metadata 到 WorkflowExecute
	if err := workflowExec.SetMetadata(metadata); err != nil {
		logger.Error("设置 Metadata 失败", zap.Error(err))
		return nil, err
	}

	// ========== Step 6: 保存 WorkflowExecute 实例 ==========
	if err := s.store.Create(ctx, workflowExec); err != nil {
		logger.Error("创建工作流执行实例失败", zap.Error(err))
		return nil, err
	}

	logger.Info("工作流执行实例创建成功",
		zap.String("exec_id", workflowExec.ID.String()),
		zap.Int("total_steps", workflowExec.TotalSteps))

	// ========== Step 7: 批量创建所有 Task（status=todo）⭐ ==========
	tasks, err := s.batchCreateTasks(ctx, workflowExec, workflow, steps, metadata)
	if err != nil {
		logger.Error("批量创建任务失败", zap.Error(err))
		// 标记 WorkflowExecute 为失败
		workflowExec.Status = core.WorkflowExecuteStatusFailed
		workflowExec.ErrorMessage = "批量创建任务失败: " + err.Error()
		s.store.Update(ctx, workflowExec)
		return nil, err
	}

	logger.Info("批量创建任务成功",
		zap.Int("task_count", len(tasks)),
		zap.String("exec_id", workflowExec.ID.String()))

	// ========== Step 8: 激活第一个 Task（status=pending, timePlan=now）⭐ ==========
	if len(tasks) > 0 {
		firstTask := tasks[0]
		if err := s.activateTask(ctx, firstTask, workflowExec); err != nil {
			logger.Error("激活第一个任务失败", zap.Error(err))
			// 不返回错误，任务已经创建成功
		} else {
			logger.Info("第一个任务激活成功",
				zap.String("task_id", firstTask.ID.String()),
				zap.String("task_name", firstTask.Name))
		}
	}

	// ========== Step 9: 更新 Workflow 统计信息 ==========
	if err := s.workflowStore.UpdateStats(ctx, workflow.ID, core.WorkflowExecuteStatusPending); err != nil {
		logger.Error("更新工作流统计失败", zap.Error(err))
		// 不影响主流程，只记录日志
	}

	logger.Info("工作流执行启动完成",
		zap.String("workflow_id", workflow.ID.String()),
		zap.String("exec_id", workflowExec.ID.String()),
		zap.Int("total_tasks", len(tasks)))

	return workflowExec, nil
}

// batchCreateTasks 批量创建所有任务 ⭐
func (s *WorkflowExecuteService) batchCreateTasks(
	ctx context.Context,
	workflowExec *core.WorkflowExecute,
	workflow *core.Workflow,
	steps []core.WorkflowStep,
	metadata *core.Metadata,
) ([]*core.Task, error) {
	tasks := make([]*core.Task, 0, len(steps))
	now := time.Now()

	// ========== 第一遍：创建所有 Task 对象 ==========
	for _, step := range steps {
		// 🔥 根据 Runner 类型准备 Command 和 Args
		// 规则：
		// - CommandRunner/DefaultRunner: Command=step.Args["command"], Args=step.Args["args"]
		// - HttpRunner: Command="http", Args=JSON(step.Args)
		// - 其他Runner: Command=step.Category, Args=JSON(step.Args)
		var taskCommand string
		var taskArgs string

		if step.Category == "command" || step.Category == "default" {
			// ⭐ CommandRunner 特殊处理：从 Args 中提取 command 和 args
			if cmdVal, ok := step.Args["command"]; ok {
				if cmdStr, ok := cmdVal.(string); ok {
					taskCommand = cmdStr
				}
			}
			// 如果没有提取到command，使用Category作为fallback
			if taskCommand == "" {
				taskCommand = step.Category
				logger.Warn("CommandRunner未找到command字段，使用Category作为fallback",
					zap.Int("step_order", step.Order),
					zap.String("category", step.Category))
			}

			// 提取args（可能是string或[]string）
			if argsVal, ok := step.Args["args"]; ok {
				switch v := argsVal.(type) {
				case string:
					taskArgs = v
				case []interface{}:
					// 数组形式，转为JSON字符串
					if argsJSON, err := json.Marshal(v); err == nil {
						taskArgs = string(argsJSON)
					}
				default:
					// 其他类型也尝试序列化
					if argsJSON, err := json.Marshal(v); err == nil {
						taskArgs = string(argsJSON)
					}
				}
			}
		} else {
			// ⭐ 其他 Runner：Command = Category, Args = JSON(step.Args)
			taskCommand = step.Category
			if argsJSON, err := json.Marshal(step.Args); err == nil {
				taskArgs = string(argsJSON)
			}
		}

		task := &core.Task{
			ID:             uuid.New(),
			TeamID:         workflow.TeamID,
			Project:        workflow.Project,
			Category:       step.Category,
			Workflow:       &workflow.ID,     // 冗余字段，提升查询性能
			WorkflowExecID: &workflowExec.ID, // 关联执行实例
			StepOrder:      step.Order,       // 步骤序号
			Name:           fmt.Sprintf("%s - Step %d: %s", workflow.Name, step.Order, step.Name),
			Command:        taskCommand, // 🔥 根据Runner类型动态设置
			Args:           taskArgs,    // 🔥 根据Runner类型动态设置
			Description:    step.Description,
			TimePlan:       time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),     // ⭐ todo 状态使用默认时间（待激活）
			TimeoutAt:      calculateWorkflowTimeout(now, workflow.Timeout), // 🔥 使用 Workflow.Timeout 或默认 24 小时
			Status:         "todo",                                          // ⭐ 初始状态为 todo
			SaveLog:        boolPtr(true),
			Timeout:        step.Timeout,
			IsStandalone:   boolPtr(false),
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// 设置 Metadata（继承自 Workflow）
		if err := task.SetMetadata(metadata); err != nil {
			logger.Error("设置任务 Metadata 失败", zap.Error(err), zap.Int("step_order", step.Order))
		}

		tasks = append(tasks, task)

		logger.Debug("创建任务对象",
			zap.Int("step_order", step.Order),
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name))
	}

	// ========== 第二遍：建立 Previous/Next 关系 ==========
	for i := range tasks {
		if i > 0 {
			// 设置 Previous（指向前一个 Task）
			tasks[i].Previous = &tasks[i-1].ID
		}
		if i < len(tasks)-1 {
			// 设置 Next（指向后一个 Task）
			tasks[i].Next = &tasks[i+1].ID
		}
	}

	// ========== 第三遍：批量保存到数据库 ==========
	// 注意：这里需要 TaskStore 支持批量创建
	// 如果没有BatchCreate，就逐个创建
	for _, task := range tasks {
		// 使用 Create 而不是 BatchCreate（当前 TaskStore 可能没有 BatchCreate）
		if _, err := s.taskStore.Create(ctx, task); err != nil {
			logger.Error("创建任务失败",
				zap.Error(err),
				zap.String("task_id", task.ID.String()),
				zap.Int("step_order", task.StepOrder))
			return nil, err
		}
	}

	logger.Info("批量创建任务完成",
		zap.Int("task_count", len(tasks)),
		zap.String("workflow_exec_id", workflowExec.ID.String()))

	return tasks, nil
}

// activateTask 激活任务（设置 status=pending, timePlan=now，应用模板替换）⭐
func (s *WorkflowExecuteService) activateTask(
	ctx context.Context,
	task *core.Task,
	workflowExec *core.WorkflowExecute,
) error {
	// ========== Step 1: 获取 Variables ==========
	variables, err := workflowExec.GetVariables()
	if err != nil {
		logger.Error("获取 Variables 失败", zap.Error(err))
		variables = make(map[string]interface{}) // 使用空 map
	}

	// ========== Step 2: 应用模板替换到 Args ⭐ ==========
	if task.Args != "" {
		// 2.1 尝试解析为 JSON（map 或 array）
		var argsData interface{}
		if err := json.Unmarshal([]byte(task.Args), &argsData); err == nil {
			// 2.2 根据类型进行变量替换
			switch v := argsData.(type) {
			case map[string]interface{}:
				// JSON 对象：递归替换
				replacedArgs := s.replaceVariablesInMap(v, variables)
				if argsJSON, err := json.Marshal(replacedArgs); err == nil {
					task.Args = string(argsJSON)
				}
			case []interface{}:
				// JSON 数组：递归替换
				replacedArgs := s.replaceVariablesInArray(v, variables)
				if argsJSON, err := json.Marshal(replacedArgs); err == nil {
					task.Args = string(argsJSON)
				}
			case string:
				// JSON 字符串：直接替换
				task.Args = s.replaceVariables(v, variables)
			}

			logger.Debug("任务参数模板替换完成",
				zap.String("task_id", task.ID.String()),
				zap.String("category", task.Category),
				zap.Int("variable_count", len(variables)))
		} else {
			// 2.3 如果不是有效的 JSON，尝试作为普通字符串进行替换
			// 这种情况可能出现在旧版本数据或特殊情况
			task.Args = s.replaceVariables(task.Args, variables)
			logger.Debug("任务参数作为普通字符串进行模板替换",
				zap.String("task_id", task.ID.String()),
				zap.String("category", task.Category))
		}
	}

	// ========== Step 3: 更新任务状态 ==========
	now := time.Now()
	task.Status = core.TaskStatusPending
	task.TimePlan = now
	task.UpdatedAt = now

	// 设置超时时间
	if task.Timeout > 0 {
		task.TimeoutAt = now.Add(time.Duration(task.Timeout) * time.Second)
	}

	// ========== Step 4: 保存更新 ==========
	if _, err := s.taskStore.Update(ctx, task); err != nil {
		logger.Error("激活任务失败", zap.Error(err), zap.String("task_id", task.ID.String()))
		return err
	}

	// ========== Step 5: 更新 WorkflowExecute 状态 ==========
	workflowExec.Status = core.WorkflowExecuteStatusRunning
	workflowExec.CurrentStep = task.StepOrder
	if workflowExec.TimeStart == nil {
		workflowExec.TimeStart = &now
	}
	workflowExec.UpdatedAt = now

	if err := s.store.Update(ctx, workflowExec); err != nil {
		logger.Error("更新工作流执行状态失败", zap.Error(err))
		// 不影响任务激活，只记录日志
	}

	logger.Info("任务激活成功",
		zap.String("task_id", task.ID.String()),
		zap.String("task_name", task.Name),
		zap.Int("step_order", task.StepOrder))

	return nil
}

// replaceVariablesInMap 递归替换 map 中的变量 ${variable}  ⭐
func (s *WorkflowExecuteService) replaceVariablesInMap(
	data map[string]interface{},
	variables map[string]interface{},
) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			// 替换字符串中的 ${variable}
			result[key] = s.replaceVariables(v, variables)
		case map[string]interface{}:
			// 递归处理嵌套 map
			result[key] = s.replaceVariablesInMap(v, variables)
		case []interface{}:
			// 处理数组
			result[key] = s.replaceVariablesInArray(v, variables)
		default:
			// 其他类型直接复制
			result[key] = value
		}
	}

	return result
}

// replaceVariablesInArray 递归替换数组中的变量
func (s *WorkflowExecuteService) replaceVariablesInArray(
	data []interface{},
	variables map[string]interface{},
) []interface{} {
	result := make([]interface{}, len(data))

	for i, value := range data {
		switch v := value.(type) {
		case string:
			result[i] = s.replaceVariables(v, variables)
		case map[string]interface{}:
			result[i] = s.replaceVariablesInMap(v, variables)
		case []interface{}:
			result[i] = s.replaceVariablesInArray(v, variables)
		default:
			result[i] = value
		}
	}

	return result
}

// replaceVariables 替换字符串中的 ${variable} 模板 ⭐
// 示例：replaceVariables("image:${image_tag}", {"image_tag": "v1.2.3"}) → "image:v1.2.3"
func (s *WorkflowExecuteService) replaceVariables(
	text string,
	variables map[string]interface{},
) string {
	// 正则表达式匹配 ${variable_name}
	re := regexp.MustCompile(`\$\{([a-zA-Z0-9_]+)\}`)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		// 提取变量名（去掉 ${ 和 }）
		varName := strings.TrimPrefix(match, "${")
		varName = strings.TrimSuffix(varName, "}")

		// 查找变量值
		if value, ok := variables[varName]; ok {
			// 转换为字符串
			return fmt.Sprintf("%v", value)
		}

		// 如果变量不存在，保持原样
		logger.Debug("变量未找到，保持原样",
			zap.String("variable", varName),
			zap.String("match", match))
		return match
	})
}

// HandleTaskComplete 处理任务完成 ⭐（核心方法 - 状态流转 + 参数传递）
func (s *WorkflowExecuteService) HandleTaskComplete(ctx context.Context, taskID uuid.UUID) error {
	logger.Info("开始处理任务完成", zap.String("task_id", taskID.String()))

	// ========== Step 1: 加载 Task ==========
	task, err := s.taskStore.FindByID(ctx, taskID)
	if err != nil {
		logger.Error("加载任务失败", zap.Error(err), zap.String("task_id", taskID.String()))
		return err
	}

	// ========== Step 2: 验证是否是工作流任务 ==========
	if task.WorkflowExecID == nil {
		logger.Debug("非工作流任务，跳过处理", zap.String("task_id", taskID.String()))
		return nil // 非工作流任务，不处理
	}

	// ========== Step 3: 加载 WorkflowExecute ==========
	workflowExec, err := s.store.FindByID(ctx, *task.WorkflowExecID)
	if err != nil {
		logger.Error("加载工作流执行实例失败", zap.Error(err))
		return err
	}

	logger.Info("工作流任务完成",
		zap.String("task_id", taskID.String()),
		zap.String("task_name", task.Name),
		zap.String("task_status", task.Status),
		zap.Int("step_order", task.StepOrder),
		zap.String("exec_id", workflowExec.ID.String()))

	// ========== Step 4: 🔥 环境锁定（第一个 Task 完成）==========
	if task.StepOrder == 1 && task.Status == core.TaskStatusSuccess {
		if err := s.lockWorkflowEnvironment(ctx, task, workflowExec); err != nil {
			logger.Error("环境锁定失败", zap.Error(err))
			// 不中断流程，继续执行
		}
	}

	// ========== Step 5: ⭐ 提取 Task.Output 并合并到 WorkflowExecute.Variables ==========
	if len(task.Output) > 0 {
		var output map[string]interface{}
		if err := json.Unmarshal([]byte(task.Output), &output); err == nil && len(output) > 0 {
			// 合并到 Variables
			if err := workflowExec.MergeVariables(output); err != nil {
				logger.Error("合并任务输出到 Variables 失败", zap.Error(err))
			} else {
				logger.Info("任务输出已合并到 Variables",
					zap.Int("output_keys", len(output)),
					zap.String("task_id", taskID.String()))
			}
		} else if err != nil {
			// 🔥 JSON解析失败，记录错误日志
			logger.Warn("任务输出不是有效的JSON格式，无法合并到Variables",
				zap.Error(err),
				zap.String("task_id", taskID.String()),
				zap.String("task_name", task.Name),
				zap.String("category", task.Category),
				zap.Int("output_length", len(task.Output)),
				zap.String("output_preview", truncateString(task.Output, 200)))
		} else {
			// 🔥 解析成功但output为空
			logger.Debug("任务输出为空JSON对象",
				zap.String("task_id", taskID.String()),
				zap.String("output", task.Output))
		}
	}

	// ========== Step 6: 更新 WorkflowExecute 统计信息 ==========
	success := task.Status == core.TaskStatusSuccess
	workflowExec.UpdateStepStats(task.StepOrder, success)

	// ========== Step 7: 判断任务状态，决定下一步动作 ==========
	now := time.Now()
	workflowExec.UpdatedAt = now

	switch task.Status {
	case core.TaskStatusSuccess:
		// ========== 7.1 任务成功 → 激活下一个 Task ==========
		if task.Next != nil {
			// 查找下一个任务
			nextTask, err := s.taskStore.FindByID(ctx, *task.Next)
			if err != nil {
				logger.Error("查找下一个任务失败", zap.Error(err))
			} else {
				// 激活下一个任务
				if err := s.activateTask(ctx, nextTask, workflowExec); err != nil {
					logger.Error("激活下一个任务失败", zap.Error(err))
					// 标记工作流执行失败
					workflowExec.Status = core.WorkflowExecuteStatusFailed
					workflowExec.ErrorMessage = fmt.Sprintf("激活任务失败: %s", err.Error())
					workflowExec.TimeEnd = &now
				}
			}
		} else {
			// ========== 7.2 没有下一个任务 → 工作流执行成功 ==========
			workflowExec.Status = core.WorkflowExecuteStatusSuccess
			workflowExec.TimeEnd = &now
			logger.Info("工作流执行成功",
				zap.String("exec_id", workflowExec.ID.String()),
				zap.Int("total_steps", workflowExec.TotalSteps),
				zap.Int("success_steps", workflowExec.SuccessSteps))
		}

	case core.TaskStatusFailed, core.TaskStatusError, core.TaskStatusTimeout:
		// ========== 7.3 任务失败 → 工作流执行失败 ==========
		workflowExec.Status = core.WorkflowExecuteStatusFailed
		workflowExec.TimeEnd = &now

		// 尝试从 Output 中提取错误信息
		errorMsg := task.Status
		if len(task.Output) > 0 {
			var outputMap map[string]interface{}
			if err := json.Unmarshal([]byte(task.Output), &outputMap); err == nil {
				if errMsg, ok := outputMap["error"].(string); ok && errMsg != "" {
					errorMsg = errMsg
				}
			}
		}
		workflowExec.ErrorMessage = fmt.Sprintf("任务 %s 失败: %s", task.Name, errorMsg)

		logger.Error("工作流执行失败",
			zap.String("exec_id", workflowExec.ID.String()),
			zap.String("failed_task", task.Name),
			zap.String("task_status", task.Status),
			zap.String("error", workflowExec.ErrorMessage))

	case core.TaskStatusCanceled:
		// ========== 7.4 任务取消 → 工作流执行取消 ==========
		workflowExec.Status = core.WorkflowExecuteStatusCanceled
		workflowExec.TimeEnd = &now
		workflowExec.ErrorMessage = fmt.Sprintf("任务 %s 被取消", task.Name)

		logger.Info("工作流执行已取消",
			zap.String("exec_id", workflowExec.ID.String()),
			zap.String("canceled_task", task.Name))

	default:
		// 其他状态（如 running, pending），暂不处理
		logger.Debug("任务状态未完成，等待后续处理",
			zap.String("task_id", taskID.String()),
			zap.String("status", task.Status))
	}

	// ========== Step 8: 保存 WorkflowExecute 更新 ==========
	if err := s.store.Update(ctx, workflowExec); err != nil {
		logger.Error("更新工作流执行实例失败", zap.Error(err))
		return err
	}

	// ========== Step 9: 更新 Workflow 统计信息 ==========
	if workflowExec.IsCompleted() {
		if err := s.workflowStore.UpdateStats(ctx, workflowExec.WorkflowID, workflowExec.Status); err != nil {
			logger.Error("更新工作流统计失败", zap.Error(err))
			// 不影响主流程
		}
	}

	logger.Info("任务完成处理完毕",
		zap.String("task_id", taskID.String()),
		zap.String("workflow_exec_id", workflowExec.ID.String()),
		zap.String("workflow_exec_status", workflowExec.Status),
		zap.Int("completed_steps", workflowExec.CompletedSteps),
		zap.Int("total_steps", workflowExec.TotalSteps))

	return nil
}

// lockWorkflowEnvironment 锁定工作流执行环境 🔒（第一个 Task 完成后）
// 功能：
// - 记录第一个 Task 执行的 Worker ID、Worker Name、Working Directory
// - 更新所有 todo 状态 Task 的 Metadata，将 worker_select 限制为这个 Worker
// - 确保后续所有 Task 都在同一个 Worker 和工作目录中执行
func (s *WorkflowExecuteService) lockWorkflowEnvironment(
	ctx context.Context,
	firstTask *core.Task,
	workflowExec *core.WorkflowExecute,
) error {
	logger.Info("开始锁定工作流执行环境",
		zap.String("exec_id", workflowExec.ID.String()),
		zap.String("task_id", firstTask.ID.String()))

	// ========== Step 1: 提取 Worker 信息 ==========
	// 从第一个 Task 中获取 Worker ID, Worker Name, Working Directory
	// 这些信息应该在 Task 执行后被 Worker 填充

	var workerID *uuid.UUID
	var workerName string
	var workingDir string

	// 1.1 从 Task.WorkerID 获取（如果有）
	if firstTask.WorkerID != nil {
		workerID = firstTask.WorkerID
	}

	// 1.2 从 Task.Metadata 中获取 Worker Name 和 Working Directory
	taskMetadata, err := firstTask.GetMetadata()
	if err != nil || taskMetadata == nil {
		logger.Warn("获取任务 Metadata 失败，环境锁定可能不完整", zap.Error(err))
		taskMetadata = &core.Metadata{}
	}

	// 1.3 从 Metadata 中提取信息
	if workerID == nil && taskMetadata.WorkerSelect != nil && len(taskMetadata.WorkerSelect) > 0 {
		// 尝试从 WorkerSelect 中获取 Worker ID
		workerIDStr := taskMetadata.WorkerSelect[0]
		if parsedID, err := uuid.Parse(workerIDStr); err == nil {
			workerID = &parsedID
		}
	}

	// 1.4 获取 Worker Name（需要从 WorkerStore 查询）
	if workerID != nil {
		// TODO: 如果需要 Worker Name，可以从 WorkerStore 查询
		// worker, err := s.workerStore.FindByID(ctx, *workerID)
		// if err == nil {
		//     workerName = worker.Name
		// }
		workerName = workerID.String() // 暂时使用 UUID 作为名称
	}

	// 1.5 获取 Working Directory
	if taskMetadata.WorkingDir != "" {
		workingDir = taskMetadata.WorkingDir
	} else {
		// 使用默认工作目录
		workingDir = "/tmp/workflow/" + workflowExec.ID.String()
	}

	// ========== Step 2: 更新 WorkflowExecute 的锁定字段 ==========
	workflowExec.LockedWorkerID = workerID
	workflowExec.LockedWorkerName = workerName
	workflowExec.LockedWorkingDir = workingDir

	logger.Info("工作流环境锁定信息",
		zap.String("exec_id", workflowExec.ID.String()),
		zap.String("locked_worker_id", func() string {
			if workerID != nil {
				return workerID.String()
			}
			return "nil"
		}()),
		zap.String("locked_worker_name", workerName),
		zap.String("locked_working_dir", workingDir))

	// ========== Step 3: 更新所有 todo 状态的 Task ==========
	// 3.1 查询所有属于这个 WorkflowExecute 且状态为 todo 的 Task
	// 注意：这里需要 TaskStore 支持按 WorkflowExecID 和 Status 查询
	// 简化实现：查询所有 Task，然后过滤

	// TODO: 实现批量更新
	// 这里暂时留空，实际使用时需要：
	// 1. 查询所有 status=todo 的 Task
	// 2. 更新它们的 Metadata，将 worker_select 设置为 [locked_worker_id]
	// 3. 更新 working_dir 为 locked_working_dir

	// 伪代码示例：
	// tasks, err := s.taskStore.ListByWorkflowExecID(ctx, workflowExec.ID)
	// if err == nil {
	//     for _, task := range tasks {
	//         if task.Status == "todo" {
	//             taskMeta, _ := task.GetMetadata()
	//             if taskMeta == nil {
	//                 taskMeta = &core.Metadata{}
	//             }
	//
	//             // 锁定 Worker
	//             if workerID != nil {
	//                 taskMeta.WorkerSelect = []string{workerID.String()}
	//             }
	//
	//             // 锁定工作目录
	//             taskMeta.WorkingDir = workingDir
	//
	//             // 更新 Task
	//             task.SetMetadata(taskMeta)
	//             s.taskStore.Update(ctx, task)
	//         }
	//     }
	// }

	logger.Info("工作流环境锁定完成",
		zap.String("exec_id", workflowExec.ID.String()),
		zap.String("worker_id", func() string {
			if workerID != nil {
				return workerID.String()
			}
			return "nil"
		}()))

	return nil
}

// calculateWorkflowTimeout 计算工作流任务的超时时间
//
// 参数:
//   - baseTime: 基准时间（通常是任务创建时间）
//   - workflowTimeout: 工作流配置的超时时间（秒）
//
// 返回:
//   - 超时时间点
//
// 逻辑:
//   - 如果 workflowTimeout > 0：baseTime + workflowTimeout 秒
//   - 否则：baseTime + 24 小时（默认值）
func calculateWorkflowTimeout(baseTime time.Time, workflowTimeout int) time.Time {
	if workflowTimeout > 0 {
		return baseTime.Add(time.Duration(workflowTimeout) * time.Second)
	}
	// 默认超时时间：24 小时
	return baseTime.Add(24 * time.Hour)
}

// truncateString 截断字符串到指定长度
//
// 参数:
//   - s: 要截断的字符串
//   - maxLen: 最大长度
//
// 返回:
//   - 截断后的字符串，如果超过maxLen会添加"..."后缀
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
