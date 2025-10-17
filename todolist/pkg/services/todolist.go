package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/codelieche/todolist/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewTodoListService 创建 TodoListService 实例
func NewTodoListService(store core.TodoListStore) core.TodoListService {
	return &TodoListService{
		store: store,
	}
}

// TodoListService 待办事项服务实现
type TodoListService struct {
	store core.TodoListStore
}

// FindByID 根据ID获取待办事项
func (s *TodoListService) FindByID(ctx context.Context, id string) (*core.TodoList, error) {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return nil, core.ErrBadRequest
	}

	return s.store.FindByID(ctx, uuidID)
}

// Create 创建待办事项
func (s *TodoListService) Create(ctx context.Context, todo *core.TodoList) (*core.TodoList, error) {
	// 验证参数
	if todo.Title == "" {
		logger.Error("todo title is required")
		return nil, core.ErrBadRequest
	}

	if todo.UserID == "" {
		logger.Error("user id is required")
		return nil, core.ErrBadRequest
	}

	// 设置默认值
	if todo.Category == "" {
		todo.Category = "general"
	}

	if todo.Status == "" {
		todo.Status = core.TodoStatusPending
	}

	if todo.Priority <= 0 {
		todo.Priority = 1
	}

	// 生成UUID
	if todo.ID == uuid.Nil {
		todo.ID = uuid.New()
	} else {
		// 如果指定了id，还需要判断id是否已经存在
		_, err := s.store.FindByIDAndUserID(ctx, todo.ID, todo.UserID)
		if err == nil {
			logger.Error("todo id already exists", zap.String("id", todo.ID.String()))
			return nil, core.ErrConflict
		} else if err != core.ErrNotFound {
			return nil, err
		}
	}

	result, err := s.store.Create(ctx, todo)
	if err != nil {
		logger.Error("create todo error", zap.Error(err))
	}
	return result, err
}

// Update 更新待办事项信息
func (s *TodoListService) Update(ctx context.Context, todo *core.TodoList) (*core.TodoList, error) {
	// 验证参数
	if todo.ID == uuid.Nil {
		logger.Error("todo id is required")
		return nil, core.ErrBadRequest
	}

	if todo.UserID == "" {
		logger.Error("user id is required")
		return nil, core.ErrBadRequest
	}

	// 检查待办事项是否存在
	existingTodo, err := s.store.FindByID(ctx, todo.ID)
	if err != nil || existingTodo.ID != todo.ID {
		logger.Error("find todo by id error", zap.Error(err), zap.String("id", todo.ID.String()))
		return nil, err
	}

	result, err := s.store.Update(ctx, todo)
	if err != nil {
		logger.Error("update todo error", zap.Error(err), zap.String("id", todo.ID.String()))
	}
	return result, err
}

// Delete 删除待办事项
func (s *TodoListService) Delete(ctx context.Context, todo *core.TodoList) error {
	if todo.ID == uuid.Nil {
		logger.Error("todo id is required")
		return core.ErrBadRequest
	}

	err := s.store.Delete(ctx, todo)
	if err != nil {
		logger.Error("delete todo error", zap.Error(err), zap.String("id", todo.ID.String()))
	}
	return err
}

// DeleteByID 根据ID删除待办事项
func (s *TodoListService) DeleteByID(ctx context.Context, id string) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	err = s.store.DeleteByID(ctx, uuidID)
	if err != nil {
		logger.Error("delete todo by id error", zap.Error(err), zap.String("id", id))
	}
	return err
}

// List 获取待办事项列表
func (s *TodoListService) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (todos []*core.TodoList, err error) {
	todos, err = s.store.List(ctx, offset, limit, filterActions...)
	if err != nil {
		logger.Error("list todos error", zap.Error(err))
	}
	return todos, err
}

// Count 统计待办事项数量
func (s *TodoListService) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	count, err := s.store.Count(ctx, filterActions...)
	if err != nil {
		logger.Error("count todos error", zap.Error(err))
	}
	return count, err
}

// Patch 动态更新待办事项字段
func (s *TodoListService) Patch(ctx context.Context, id string, updates map[string]interface{}) error {
	// 解析UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	// 验证字段有效性 - 定义哪些字段可以被修改
	validFields := map[string]bool{
		"parent_id":   true,
		"title":       true,
		"description": true,
		"status":      true,
		"priority":    true,
		"category":    true,
		"tags":        true,
		"start_time":  true, // 🔥 新增：开始时间
		"deadline":    true,
		"progress":    true, // 🔥 新增：手动完成进度
		"finished_at": true,
		"metadata":    true,
	}

	// 过滤出有效的更新字段
	var needUpdates map[string]interface{} = map[string]interface{}{}
	for field := range updates {
		if _, ok := validFields[field]; !ok {
			logger.Error("invalid todo field", zap.String("field", field))
			// 传递了不可更新的字段，我们跳过即可，不需要报错
		} else {
			needUpdates[field] = updates[field]
		}
	}

	// 🔥🔥 验证状态字段并处理自动设置时间的逻辑
	if status, ok := needUpdates["status"]; ok {
		if statusStr, isString := status.(string); isString {
			validStatus := map[string]bool{
				core.TodoStatusPending:  true,
				core.TodoStatusRunning:  true,
				core.TodoStatusDone:     true,
				core.TodoStatusCanceled: true,
			}
			if _, valid := validStatus[statusStr]; !valid {
				logger.Error("invalid todo status", zap.String("status", statusStr))
				return core.ErrBadRequest
			}

			// 🔥 场景 1：状态改为 running
			if statusStr == core.TodoStatusRunning {
				// 如果没有传递 start_time，则自动设置为当前时间
				if _, hasStartTime := needUpdates["start_time"]; !hasStartTime {
					// 需要先查询任务，判断是否已有 start_time
					existingTodo, err := s.store.FindByID(ctx, uuidID)
					if err != nil {
						logger.Error("find todo error", zap.Error(err), zap.String("id", id))
						return err
					}
					// 如果任务原本没有 start_time，则自动设置
					if existingTodo.StartTime == nil {
						now := time.Now()
						needUpdates["start_time"] = &now
						logger.Debug("auto set start_time for running status", zap.String("id", id))
					}
				}
				// 清空完成时间（如果没有明确设置）
				if _, hasFinishedAt := needUpdates["finished_at"]; !hasFinishedAt {
					needUpdates["finished_at"] = nil
				}
			}

			// 🔥 场景 2：状态改为 done
			if statusStr == core.TodoStatusDone {
				// 2.1 如果没有传递 finished_at，则自动设置为当前时间
				if _, hasFinishedAt := needUpdates["finished_at"]; !hasFinishedAt {
					now := time.Now()
					needUpdates["finished_at"] = &now
					logger.Debug("auto set finished_at for done status", zap.String("id", id))
				}

				// 2.2 🔥 如果没有 start_time，则自动设置（从 pending 直接完成的场景）
				if _, hasStartTime := needUpdates["start_time"]; !hasStartTime {
					// 需要先查询任务，判断是否已有 start_time
					existingTodo, err := s.store.FindByID(ctx, uuidID)
					if err != nil {
						logger.Error("find todo error", zap.Error(err), zap.String("id", id))
						return err
					}
					// 如果任务原本没有 start_time，则自动设置（与 finished_at 相同时间）
					if existingTodo.StartTime == nil {
						if finishedAt, ok := needUpdates["finished_at"].(*time.Time); ok && finishedAt != nil {
							needUpdates["start_time"] = finishedAt
							logger.Debug("auto set start_time (same as finished_at) for done status from pending",
								zap.String("id", id))
						}
					}
				}

				// 2.3 🔥 自动设置 progress 为 100（如果有子任务除外）
				existingTodo, err := s.store.FindByID(ctx, uuidID)
				if err == nil && existingTodo.ChildrenCount == 0 {
					// 无子任务的任务，自动设置 progress 为 100
					progress := 100
					needUpdates["progress"] = &progress
					logger.Debug("auto set progress to 100 for done status", zap.String("id", id))
				}
			}

			// 🔥 场景 3：状态改为 pending 或 canceled
			if statusStr == core.TodoStatusPending || statusStr == core.TodoStatusCanceled {
				// 清空完成时间（如果没有明确设置）
				if _, hasFinishedAt := needUpdates["finished_at"]; !hasFinishedAt {
					needUpdates["finished_at"] = nil
				}
			}
		}
	}

	// 🔥🔥 验证 progress 字段（0-100）
	if progress, ok := needUpdates["progress"]; ok {
		var progressValue int
		var validProgress bool

		// 处理多种数字类型
		switch v := progress.(type) {
		case int:
			progressValue = v
			validProgress = true
		case float64:
			progressValue = int(v)
			validProgress = true
		case *int:
			if v != nil {
				progressValue = *v
				validProgress = true
			}
		}

		if validProgress {
			if progressValue < 0 || progressValue > 100 {
				logger.Error("invalid progress value", zap.Int("progress", progressValue))
				return core.ErrBadRequest
			}

			// 🔥 检查是否有子任务（有子任务的任务不允许手动设置进度）
			existingTodo, err := s.store.FindByID(ctx, uuidID)
			if err != nil {
				logger.Error("find todo error", zap.Error(err), zap.String("id", id))
				return err
			}
			if existingTodo.ChildrenCount > 0 {
				logger.Error("cannot set progress for todo with children",
					zap.String("id", id),
					zap.Int("children_count", existingTodo.ChildrenCount))
				return core.ErrBadRequest
			}

			// 🔥🔥 智能状态切换：进度达到 100% 自动完成任务
			if progressValue == 100 && existingTodo.Status != core.TodoStatusDone {
				now := time.Now()
				needUpdates["status"] = core.TodoStatusDone
				needUpdates["finished_at"] = &now

				// 🔥 如果没有 start_time，自动设置（从 pending 直接完成的场景）
				if existingTodo.StartTime == nil {
					needUpdates["start_time"] = &now
				}

				logger.Info("auto mark task as done when progress reaches 100%",
					zap.String("id", id),
					zap.String("title", existingTodo.Title))
			}

			// 🔥🔥 反向逻辑：进度 < 100% 且任务已完成，取消完成状态
			if progressValue < 100 && existingTodo.Status == core.TodoStatusDone {
				needUpdates["status"] = core.TodoStatusPending
				needUpdates["finished_at"] = nil

				logger.Info("auto revert task from done to pending when progress < 100%",
					zap.String("id", id),
					zap.Int("progress", progressValue))
			}
		}
	}

	// 处理 metadata 字段的特殊转换
	if metadata, ok := needUpdates["metadata"]; ok {
		if metadataMap, isMap := metadata.(map[string]interface{}); isMap {
			// 将 map[string]interface{} 转换为 json.RawMessage
			if len(metadataMap) == 0 {
				// 空对象转换为空的 JSON 对象
				needUpdates["metadata"] = json.RawMessage(`{}`)
			} else {
				// 非空对象序列化为 JSON
				jsonData, err := json.Marshal(metadataMap)
				if err != nil {
					logger.Error("marshal metadata error", zap.Error(err))
					return core.ErrBadRequest
				}
				needUpdates["metadata"] = json.RawMessage(jsonData)
			}
			logger.Debug("converted metadata to json.RawMessage", zap.String("id", id))
		} else if metadata == nil {
			// 如果传入 null，则设置为 nil
			needUpdates["metadata"] = nil
			logger.Debug("set metadata to nil", zap.String("id", id))
		}
		// 如果已经是 json.RawMessage 类型，则不需要处理
	}

	// 调用store的Patch方法进行更新
	err = s.store.Patch(ctx, uuidID, needUpdates)
	if err != nil {
		logger.Error("patch todo error", zap.Error(err), zap.String("id", id))
	}
	return err
}

// GetUserTodos 获取用户的待办事项列表
func (s *TodoListService) GetUserTodos(ctx context.Context, userID string, offset int, limit int, filterActions ...filters.Filter) (todos []*core.TodoList, err error) {
	// 添加用户ID过滤器
	userFilter := &filters.FilterOption{
		Column: "user_id",
		Value:  userID,
		Op:     filters.FILTER_EQ,
	}

	// 将用户过滤器添加到过滤器列表的前面
	allFilters := []filters.Filter{userFilter}
	allFilters = append(allFilters, filterActions...)

	return s.List(ctx, offset, limit, allFilters...)
}

// CountUserTodos 统计用户的待办事项数量
func (s *TodoListService) CountUserTodos(ctx context.Context, userID string, filterActions ...filters.Filter) (int64, error) {
	// 添加用户ID过滤器
	userFilter := &filters.FilterOption{
		Column: "user_id",
		Value:  userID,
		Op:     filters.FILTER_EQ,
	}

	// 将用户过滤器添加到过滤器列表的前面
	allFilters := []filters.Filter{userFilter}
	allFilters = append(allFilters, filterActions...)

	return s.Count(ctx, allFilters...)
}

// MarkDone 标记待办事项为已完成
// 🔥 自动处理逻辑：
// 1. 设置 finished_at 为当前时间
// 2. 如果没有 start_time，自动设置（与 finished_at 相同）
// 3. 如果无子任务，自动设置 progress 为 100
func (s *TodoListService) MarkDone(ctx context.Context, id string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":      core.TodoStatusDone,
		"finished_at": &now,
	}
	// 🔥 Patch 方法会自动处理 start_time 和 progress
	return s.Patch(ctx, id, updates)
}

// MarkRunning 标记待办事项为进行中
// 🔥 自动处理逻辑：
// 1. 如果没有 start_time，自动设置为当前时间
// 2. 清空 finished_at
func (s *TodoListService) MarkRunning(ctx context.Context, id string) error {
	updates := map[string]interface{}{
		"status":      core.TodoStatusRunning,
		"finished_at": nil,
	}
	// 🔥 Patch 方法会自动处理 start_time
	return s.Patch(ctx, id, updates)
}

// MarkPending 标记待办事项为待办
func (s *TodoListService) MarkPending(ctx context.Context, id string) error {
	updates := map[string]interface{}{
		"status":      core.TodoStatusPending,
		"finished_at": nil,
	}
	return s.Patch(ctx, id, updates)
}

// MarkCanceled 标记待办事项为已取消
func (s *TodoListService) MarkCanceled(ctx context.Context, id string) error {
	updates := map[string]interface{}{
		"status":      core.TodoStatusCanceled,
		"finished_at": nil,
	}
	return s.Patch(ctx, id, updates)
}

// GetChildTodos 获取子任务列表
// 🔥 用于详情页展示所有子任务（不分页，限制100条）
func (s *TodoListService) GetChildTodos(ctx context.Context, parentID string) ([]*core.TodoList, error) {
	// 解析UUID
	parentUUID, err := uuid.Parse(parentID)
	if err != nil {
		logger.Error("parse parent id error", zap.Error(err), zap.String("parent_id", parentID))
		return nil, core.ErrBadRequest
	}

	// 构建过滤器：parent_id = parentUUID
	parentFilter := &filters.FilterOption{
		Column: "parent_id",
		Value:  parentUUID,
		Op:     filters.FILTER_EQ,
	}

	// 🔥 不分页，限制100条，按创建时间排序
	return s.store.List(ctx, 0, 100, parentFilter)
}

// RecalculateChildrenStats 重新计算子任务统计（修复不一致数据）
// 🔥 用于数据修复接口，当统计字段不准确时调用
func (s *TodoListService) RecalculateChildrenStats(ctx context.Context, parentID string) error {
	// 解析UUID
	parentUUID, err := uuid.Parse(parentID)
	if err != nil {
		logger.Error("parse parent id error", zap.Error(err), zap.String("parent_id", parentID))
		return core.ErrBadRequest
	}

	// 构建过滤器：parent_id = parentUUID
	allFilters := []filters.Filter{
		&filters.FilterOption{
			Column: "parent_id",
			Value:  parentUUID,
			Op:     filters.FILTER_EQ,
		},
	}

	// 查询总数
	totalCount, err := s.store.Count(ctx, allFilters...)
	if err != nil {
		return err
	}

	// 查询已完成数
	doneFilters := []filters.Filter{
		&filters.FilterOption{
			Column: "parent_id",
			Value:  parentUUID,
			Op:     filters.FILTER_EQ,
		},
		&filters.FilterOption{
			Column: "status",
			Value:  core.TodoStatusDone,
			Op:     filters.FILTER_EQ,
		},
	}
	doneCount, err := s.store.Count(ctx, doneFilters...)
	if err != nil {
		return err
	}

	// 更新父任务的统计字段
	updates := map[string]interface{}{
		"children_count": totalCount,
		"children_done":  doneCount,
	}

	return s.store.Patch(ctx, parentUUID, updates)
}

// MarkDoneWithChildren 标记任务及其所有子任务为已完成（批量操作）
// 🔥 业务场景：用户点击"完成任务"时，自动将所有子任务也标记为完成
// 委托给 Store 层实现（Store 层使用事务）
func (s *TodoListService) MarkDoneWithChildren(ctx context.Context, id string) error {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		logger.Error("parse id error", zap.Error(err), zap.String("id", id))
		return core.ErrBadRequest
	}

	return s.store.MarkDoneWithChildren(ctx, uuidID)
}

// GetTodosByTimeRange 获取时间区间内的待办事项（日历视图专用）
// 🔥 使用 OR 逻辑查询：start_time 在区间内 OR deadline 在区间内 OR 跨区间任务
func (s *TodoListService) GetTodosByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, offset, limit int, otherFilters ...filters.Filter) ([]*core.TodoList, error) {
	// 参数验证
	if userID == "" {
		logger.Error("user id is required")
		return nil, core.ErrBadRequest
	}

	if startTime.IsZero() || endTime.IsZero() {
		logger.Error("start time and end time are required")
		return nil, core.ErrBadRequest
	}

	if startTime.After(endTime) {
		logger.Error("start time must be before end time")
		return nil, core.ErrBadRequest
	}

	return s.store.GetByTimeRange(ctx, userID, startTime, endTime, offset, limit, otherFilters...)
}

// CountTodosByTimeRange 统计时间区间内的待办事项数量
func (s *TodoListService) CountTodosByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, otherFilters ...filters.Filter) (int64, error) {
	// 参数验证
	if userID == "" {
		logger.Error("user id is required")
		return 0, core.ErrBadRequest
	}

	if startTime.IsZero() || endTime.IsZero() {
		logger.Error("start time and end time are required")
		return 0, core.ErrBadRequest
	}

	if startTime.After(endTime) {
		logger.Error("start time must be before end time")
		return 0, core.ErrBadRequest
	}

	return s.store.CountByTimeRange(ctx, userID, startTime, endTime, otherFilters...)
}
