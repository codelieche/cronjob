package store

import (
	"context"
	"errors"
	"time"

	"github.com/codelieche/todolist/pkg/core"
	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewTodoListStore 创建 TodoListStore 实例
func NewTodoListStore(db *gorm.DB) core.TodoListStore {
	return &TodoListStore{
		db: db,
	}
}

// TodoListStore 待办事项存储实现
type TodoListStore struct {
	db *gorm.DB
}

// FindByID 根据ID获取待办事项
func (s *TodoListStore) FindByID(ctx context.Context, id uuid.UUID) (*core.TodoList, error) {
	var todo = &core.TodoList{}
	if err := s.db.Find(todo, "id=?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	} else {
		if todo.ID != uuid.Nil {
			return todo, nil
		} else {
			return nil, core.ErrNotFound
		}
	}
}

// FindByIDAndUserID 根据ID和用户ID获取待办事项
func (s *TodoListStore) FindByIDAndUserID(ctx context.Context, id uuid.UUID, userID string) (*core.TodoList, error) {
	var todo = &core.TodoList{}
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(todo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return todo, nil
}

// Create 创建待办事项
func (s *TodoListStore) Create(ctx context.Context, todo *core.TodoList) (*core.TodoList, error) {
	// 生成UUID
	if todo.ID == uuid.Nil {
		todo.ID = uuid.New()
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

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(todo).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回创建后的对象
		return todo, nil
	}
}

// Update 更新待办事项信息
func (s *TodoListStore) Update(ctx context.Context, todo *core.TodoList) (*core.TodoList, error) {
	if todo.ID == uuid.Nil {
		err := errors.New("传入的ID无效")
		return nil, err
	}

	// 检查待办事项是否存在
	existingTodo, err := s.FindByID(ctx, todo.ID)
	if err != nil {
		return nil, err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新待办事项信息
	if err := tx.Model(existingTodo).Updates(todo).Error; err != nil {
		tx.Rollback()
		return nil, err
	} else {
		tx.Commit()
		// 返回更新后的对象
		return existingTodo, nil
	}
}

// Delete 删除待办事项
func (s *TodoListStore) Delete(ctx context.Context, todo *core.TodoList) error {
	if todo.ID == uuid.Nil {
		return errors.New("传入的待办事项ID无效")
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Delete(todo).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// DeleteByID 根据ID删除待办事项
func (s *TodoListStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	// 先获取待办事项
	todo, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 删除待办事项
	return s.Delete(ctx, todo)
}

// DeleteByIDAndUserID 根据ID和用户ID删除待办事项
func (s *TodoListStore) DeleteByIDAndUserID(ctx context.Context, id uuid.UUID, userID string) error {
	// 先获取待办事项
	todo, err := s.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return err
	}

	// 删除待办事项
	return s.Delete(ctx, todo)
}

// List 获取待办事项列表
func (s *TodoListStore) List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (todos []*core.TodoList, err error) {
	tx := s.db.Model(&core.TodoList{})

	// 应用过滤器
	for _, action := range filterActions {
		tx = action.Filter(tx)
	}

	// 分页
	tx = tx.Offset(offset).Limit(limit)

	// 获取列表
	if err = tx.Find(&todos).Error; err != nil {
		return nil, err
	}

	return todos, nil
}

// Count 统计待办事项数量
func (s *TodoListStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	var count int64
	tx := s.db.Model(&core.TodoList{})

	// 应用过滤器
	for _, action := range filterActions {
		tx = action.Filter(tx)
	}

	// 统计数量
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Patch 动态更新待办事项字段
func (s *TodoListStore) Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	// 检查ID是否有效
	if id == uuid.Nil {
		return errors.New("传入的ID无效")
	}

	// 检查待办事项是否存在
	todo, err := s.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 使用map动态更新待办事项字段
	if err := tx.Model(todo).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// PatchByUserID 根据用户ID动态更新待办事项字段
func (s *TodoListStore) PatchByUserID(ctx context.Context, id uuid.UUID, userID string, updates map[string]interface{}) error {
	// 检查ID是否有效
	if id == uuid.Nil {
		return errors.New("传入的ID无效")
	}

	// 检查待办事项是否存在且属于该用户
	todo, err := s.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return err
	}

	// 在事务中执行
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 使用map动态更新待办事项字段
	if err := tx.Model(todo).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	} else {
		tx.Commit()
		return nil
	}
}

// MarkDoneWithChildren 批量完成任务及其所有子任务（使用事务）
// 🔥 业务场景：用户点击"完成任务"时，自动将所有子任务也标记为完成
func (s *TodoListStore) MarkDoneWithChildren(ctx context.Context, id uuid.UUID) error {
	// 🔥 使用 GORM Transaction（关键！）
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// 1. 标记父任务为已完成
		if err := tx.Model(&core.TodoList{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status":      core.TodoStatusDone,
				"finished_at": &now,
			}).Error; err != nil {
			return err
		}

		// 2. 批量标记所有子任务为已完成
		if err := tx.Model(&core.TodoList{}).
			Where("parent_id = ? AND status != ?", id, core.TodoStatusDone).
			Updates(map[string]interface{}{
				"status":      core.TodoStatusDone,
				"finished_at": &now,
			}).Error; err != nil {
			return err
		}

		// 3. 更新父任务的统计字段（所有子任务都已完成）
		var childrenCount int64
		tx.Model(&core.TodoList{}).
			Where("parent_id = ?", id).
			Count(&childrenCount)

		return tx.Model(&core.TodoList{}).
			Where("id = ?", id).
			Update("children_done", childrenCount).Error
	})
}

// GetByTimeRange 获取时间区间内的待办事项（日历视图专用，使用 OR 逻辑）
// 🔥 查询逻辑：
// 1. start_time 在区间内（包括只有 start_time 的任务）
// 2. deadline 在区间内（包括只有 deadline 的任务）
// 3. 跨区间任务（start_time < 区间开始 且 deadline > 区间结束）
func (s *TodoListStore) GetByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, offset, limit int, otherFilters ...filters.Filter) ([]*core.TodoList, error) {
	var todos []*core.TodoList

	// 构建基础查询
	tx := s.db.Model(&core.TodoList{}).Where("user_id = ?", userID)

	// 🔥🔥 时间区间 OR 查询（核心逻辑）
	tx = tx.Where(
		s.db.Where("start_time >= ? AND start_time <= ?", startTime, endTime). // 开始时间在区间内
											Or("deadline >= ? AND deadline <= ?", startTime, endTime). // 截止时间在区间内
											Or("start_time < ? AND deadline > ?", startTime, endTime), // 跨区间任务
	)

	// 应用其他过滤器（团队、状态等）
	for _, filter := range otherFilters {
		if filter != nil { // 🔥 防止 nil pointer panic
			tx = filter.Filter(tx)
		}
	}

	// 排序和分页
	tx = tx.Order("COALESCE(start_time, deadline) ASC, created_at DESC"). // 优先按开始时间排序
										Offset(offset).
										Limit(limit)

	// 执行查询
	if err := tx.Find(&todos).Error; err != nil {
		return nil, err
	}

	return todos, nil
}

// CountByTimeRange 统计时间区间内的待办事项数量
func (s *TodoListStore) CountByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, otherFilters ...filters.Filter) (int64, error) {
	var count int64

	// 构建基础查询
	tx := s.db.Model(&core.TodoList{}).Where("user_id = ?", userID)

	// 🔥🔥 时间区间 OR 查询（与 GetByTimeRange 逻辑一致）
	tx = tx.Where(
		s.db.Where("start_time >= ? AND start_time <= ?", startTime, endTime).
			Or("deadline >= ? AND deadline <= ?", startTime, endTime).
			Or("start_time < ? AND deadline > ?", startTime, endTime),
	)

	// 应用其他过滤器
	for _, filter := range otherFilters {
		if filter != nil { // 🔥 防止 nil pointer panic
			tx = filter.Filter(tx)
		}
	}

	// 执行统计
	if err := tx.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}
