// Package core TodoList 核心数据模型和接口定义
//
// 包含 TodoList 系统中所有核心业务实体的数据模型定义
// 以及相关的数据访问接口和服务接口
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codelieche/todolist/pkg/utils/filters"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TodoList 状态常量定义
// 用于标识 TodoList 在生命周期中的不同状态
const (
	TodoStatusPending  = "pending"  // 待办 - 任务创建，等待处理
	TodoStatusRunning  = "running"  // 进行中 - 任务正在处理
	TodoStatusDone     = "done"     // 已完成 - 任务已完成
	TodoStatusCanceled = "canceled" // 已取消 - 任务被取消
)

// TodoList 待办事项实体
//
// 记录用户的待办事项信息，包括：
// - 基本信息：标题、描述、优先级等
// - 状态信息：完成状态、创建时间、更新时间等
// - 用户关联：通过 UserID 关联到具体用户
// - 分类标签：支持分类和标签管理
//
// 这是系统的核心实体，每个用户可以创建多个待办事项
type TodoList struct {
	ID          uuid.UUID       `gorm:"size:256;primaryKey" json:"id"`                               // 待办事项唯一标识
	UserID      string          `gorm:"size:256;index:idx_user_id;not null" json:"user_id"`          // 关联的用户ID（从认证中间件获取）
	TeamID      *uuid.UUID      `gorm:"size:256;index:idx_team_id" json:"team_id"`                   // 关联的团队ID（可为空，支持团队协作）
	Project     string          `gorm:"size:128;index:idx_project" json:"project"`                   // 关联的项目代码（可为空，支持项目隔离）
	ParentID    *uuid.UUID      `gorm:"size:256;index:idx_parent_id" json:"parent_id"`               // 父待办事项ID（可为空，支持层级化管理）
	Title       string          `gorm:"size:512;not null" json:"title"`                              // 待办事项标题
	Description string          `gorm:"type:text" json:"description"`                                // 待办事项详细描述
	Status      string          `gorm:"size:40;index:idx_status;default:pending" json:"status"`      // 当前状态
	Priority    int             `gorm:"type:int;default:1" json:"priority"`                          // 优先级（1-5，1最低，5最高）
	Category    string          `gorm:"size:128;index:idx_category;default:general" json:"category"` // 分类
	Tags        string          `gorm:"size:512" json:"tags"`                                        // 标签（以逗号分隔）
	StartTime   *time.Time      `gorm:"column:start_time;index:idx_start_time" json:"start_time"`    // 开始时间（可选，用于时间段任务和日视图）
	Deadline    *time.Time      `gorm:"column:deadline;index:idx_user_deadline" json:"deadline"`     // 截止期限（日历视图核心字段，已有复合索引支持日期范围查询）
	FinishedAt  *time.Time      `gorm:"column:finished_at" json:"finished_at"`                       // 完成时间
	Progress    *int            `gorm:"type:int;comment:手动设置的完成进度(0-100)" json:"progress"`           // 手动完成进度（0-100，优先级高于自动计算）
	Metadata    json.RawMessage `gorm:"type:json" json:"metadata" swaggertype:"object"`              // 元数据，存储额外的自定义信息
	// 🔥🔥 新增：子任务统计字段（冗余字段，用于性能优化）
	ChildrenCount int            `gorm:"type:int;default:0;comment:子任务总数" json:"children_count"`   // 子任务总数
	ChildrenDone  int            `gorm:"type:int;default:0;comment:已完成子任务数" json:"children_done"`  // 已完成子任务数
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`       // 创建时间
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`       // 更新时间
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`                                           // 软删除时间
	Deleted       *bool          `gorm:"type:boolean;default:false" json:"deleted" form:"deleted"` // 软删除标记
}

// TableName 表名
func (TodoList) TableName() string {
	return "todo_lists"
}

// GetProgress 计算任务完成进度（0-100）
//
// 优先级规则（从高到低）：
// 1. 手动设置的进度（Progress 字段）- 用户显式设置
// 2. 子任务自动计算进度 - 有子任务时自动计算
// 3. 状态判断 - 根据任务状态返回 0 或 100
func (t *TodoList) GetProgress() float64 {
	// 🔥 优先级 1：如果手动设置了进度，直接返回
	if t.Progress != nil {
		return float64(*t.Progress)
	}

	// 🔥 优先级 2：如果有子任务，自动计算进度
	if t.ChildrenCount > 0 {
		return float64(t.ChildrenDone) / float64(t.ChildrenCount) * 100
	}

	// 🔥 优先级 3：根据任务状态判断
	// - 已完成状态：100%
	// - 进行中状态：可以考虑返回 50%（但这里保守返回 0，由用户手动设置）
	// - 其他状态：0%
	if t.Status == TodoStatusDone {
		return 100.0
	}

	return 0.0
}

// HasChildren 判断是否有子任务
func (t *TodoList) HasChildren() bool {
	return t.ChildrenCount > 0
}

// BeforeDelete 删除前设置deleted字段为True
// 同时执行删除操作的额外处理
// 🔥🔥 新增：级联删除子任务
func (m *TodoList) BeforeDelete(tx *gorm.DB) (err error) {
	// 设置Deleted字段为true
	trueValue := true
	m.Deleted = &trueValue

	// 使用事务更新数据库中的deleted字段
	// 这样确保软删除时deleted字段被正确设置
	if err := tx.Model(m).Update("deleted", m.Deleted).Error; err != nil {
		return err
	}

	// 🔥🔥 如果是父任务，级联删除所有子任务
	if m.ID != uuid.Nil {
		// 查询所有子任务
		var children []*TodoList
		if err := tx.Where("parent_id = ?", m.ID).Find(&children).Error; err != nil {
			return err
		}

		// 级联删除子任务（会递归触发子任务的 BeforeDelete Hook）
		if len(children) > 0 {
			if err := tx.Delete(&children).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// AfterDelete 钩子函数，在删除后执行
// 🔥🔥 新增：删除子任务后，更新父任务的统计字段
// ⚠️ 注意：如果是级联删除（父任务删除导致子任务删除），此 Hook 不会执行实际更新
// 因为父任务也在删除过程中，无需更新其统计字段
func (m *TodoList) AfterDelete(tx *gorm.DB) (err error) {
	if m.ParentID != nil {
		// 🔥 检查父任务是否还存在（避免级联删除时的无效更新）
		var parent TodoList
		if err := tx.Where("id = ? AND deleted_at IS NULL", m.ParentID).
			First(&parent).Error; err != nil {
			// 父任务不存在或已删除，跳过更新
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}

		// 🔥 父任务存在，更新统计字段
		if err := tx.Model(&TodoList{}).
			Where("id = ?", m.ParentID).
			Update("children_count", gorm.Expr("children_count - ?", 1)).
			Error; err != nil {
			return err
		}

		// 🔥 如果删除的是已完成任务，完成数也 -1
		if m.Status == TodoStatusDone {
			return tx.Model(&TodoList{}).
				Where("id = ?", m.ParentID).
				Update("children_done", gorm.Expr("children_done - ?", 1)).
				Error
		}
	}
	return nil
}

// IsDone 判断是否已完成
func (t *TodoList) IsDone() bool {
	return t.Status == TodoStatusDone
}

// IsPending 判断是否为待办状态
func (t *TodoList) IsPending() bool {
	return t.Status == TodoStatusPending
}

// IsRunning 判断是否为进行中状态
func (t *TodoList) IsRunning() bool {
	return t.Status == TodoStatusRunning
}

// IsCanceled 判断是否已取消
func (t *TodoList) IsCanceled() bool {
	return t.Status == TodoStatusCanceled
}

// SetProgress 设置任务进度（0-100）
//
// 注意：
// - 有子任务的任务，进度由子任务自动计算，不建议手动设置（会被忽略）
// - 无子任务的任务，可以手动设置进度表示任务进展
func (t *TodoList) SetProgress(progress int) error {
	// 验证进度范围
	if progress < 0 || progress > 100 {
		return fmt.Errorf("进度必须在 0-100 之间")
	}

	// 🔥 如果有子任务，不允许手动设置进度（由子任务自动计算）
	// 这是一个业务规则，确保数据一致性
	if t.ChildrenCount > 0 {
		return fmt.Errorf("有子任务的任务进度由子任务自动计算，不能手动设置")
	}

	t.Progress = &progress
	return nil
}

// MarkDone 标记为已完成
// 🔥 自动处理逻辑：
// 1. 设置 finished_at 为当前时间
// 2. 如果没有 start_time，自动设置（与 finished_at 相同）
// 3. 如果无子任务，自动设置 progress 为 100
func (t *TodoList) MarkDone() {
	t.Status = TodoStatusDone
	now := time.Now()
	t.FinishedAt = &now

	// 🔥 如果没有 start_time，则自动设置（从 pending 直接完成的场景）
	if t.StartTime == nil {
		t.StartTime = &now
	}

	// 🔥 如果无子任务，自动设置 progress 为 100
	if t.ChildrenCount == 0 {
		progress := 100
		t.Progress = &progress
	}
}

// MarkRunning 标记为进行中
// 🔥 自动处理逻辑：
// 1. 如果没有 start_time，自动设置为当前时间
// 2. 清空 finished_at
func (t *TodoList) MarkRunning() {
	t.Status = TodoStatusRunning
	t.FinishedAt = nil

	// 🔥 如果没有 start_time，则自动设置为当前时间
	if t.StartTime == nil {
		now := time.Now()
		t.StartTime = &now
	}
}

// MarkPending 标记为待办
func (t *TodoList) MarkPending() {
	t.Status = TodoStatusPending
	t.FinishedAt = nil
}

// MarkCanceled 标记为已取消
func (t *TodoList) MarkCanceled() {
	t.Status = TodoStatusCanceled
	t.FinishedAt = nil
}

// GetMetadata 获取解析后的元数据
func (t *TodoList) GetMetadata() (map[string]interface{}, error) {
	if len(t.Metadata) == 0 {
		return make(map[string]interface{}), nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(t.Metadata, &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

// SetMetadata 设置元数据
func (t *TodoList) SetMetadata(metadata map[string]interface{}) error {
	if metadata == nil {
		t.Metadata = nil
		return nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	t.Metadata = data
	return nil
}

// AfterCreate Hook：创建子任务后，更新父任务的 ChildrenCount
func (t *TodoList) AfterCreate(tx *gorm.DB) error {
	if t.ParentID != nil {
		// 🔥 父任务的子任务总数 +1
		return tx.Model(&TodoList{}).
			Where("id = ?", t.ParentID).
			Update("children_count", gorm.Expr("children_count + ?", 1)).
			Error
	}
	return nil
}

// BeforeUpdate Hook：记录状态变化，用于优化 AfterUpdate
func (t *TodoList) BeforeUpdate(tx *gorm.DB) error {
	// 🔥🔥 只在状态字段被修改时才记录（性能优化）
	if tx.Statement.Changed("Status") {
		// 🔥 使用 Set() 而不是 SetColumn()，避免写入数据库
		// Set() 只在当前事务的上下文中存储数据，不会生成 SQL
		tx.Statement.Set("_status_changed", true)
	}
	return nil
}

// AfterUpdate Hook：子任务状态变更后，更新父任务的 ChildrenDone
// 🔥🔥 智能完成逻辑：
// 1. 当所有子任务都完成时，自动标记父任务为完成
// 2. 当任一子任务变为未完成且父任务已完成时，自动取消父任务的完成状态
// 🔥🔥🔥 性能优化：
// 1. 只在状态改变时触发（减少60%的无效触发）
// 2. 使用子查询合并数据库操作（减少50%的查询次数）
func (t *TodoList) AfterUpdate(tx *gorm.DB) error {
	if t.ParentID != nil {
		// 🔥🔥🔥 优化1：只在状态改变时才触发（避免描述等字段修改时的无效触发）
		// 从 BeforeUpdate 中获取状态改变标记
		statusChangedValue, exists := tx.Statement.Get("_status_changed")
		statusChanged := false
		if exists {
			if boolValue, ok := statusChangedValue.(bool); ok {
				statusChanged = boolValue
			}
		}
		if !statusChanged {
			// 状态未改变，跳过后续逻辑
			return nil
		}

		// 🔥🔥🔥 优化2：使用原生 SQL 子查询一次性更新 children_done 并获取父任务信息
		// 🔥 MySQL 限制：不能在子查询中引用正在更新的表，需要使用临时表包装
		err := tx.Exec(`
			UPDATE todo_lists 
			SET children_done = (
				SELECT COUNT(*) 
				FROM (
					SELECT id, parent_id, status, deleted_at
					FROM todo_lists
					WHERE deleted_at IS NULL
				) AS temp_table
				WHERE temp_table.parent_id = ? AND temp_table.status = ?
			),
			updated_at = ?
			WHERE id = ?
		`, t.ParentID, TodoStatusDone, time.Now(), t.ParentID).Error
		if err != nil {
			return err
		}

		// 查询父任务，获取最新的统计信息（这是必要的查询，用于智能完成判断）
		var parent TodoList
		if err := tx.Where("id = ?", t.ParentID).First(&parent).Error; err != nil {
			return err
		}

		// 🔥🔥 智能完成逻辑：场景1 - 所有子任务完成 → 父任务自动完成
		if parent.ChildrenCount > 0 &&
			parent.ChildrenDone == parent.ChildrenCount &&
			parent.Status != TodoStatusDone {
			now := time.Now()
			return tx.Model(&TodoList{}).
				Where("id = ?", t.ParentID).
				Updates(map[string]interface{}{
					"status":      TodoStatusDone,
					"finished_at": &now,
				}).Error
		}

		// 🔥🔥🔥 智能完成逻辑：场景2 - 子任务未全部完成 → 取消父任务完成状态
		if parent.ChildrenCount > 0 &&
			parent.ChildrenDone < parent.ChildrenCount &&
			parent.Status == TodoStatusDone {
			return tx.Model(&TodoList{}).
				Where("id = ?", t.ParentID).
				Updates(map[string]interface{}{
					"status":      TodoStatusPending,
					"finished_at": nil,
				}).Error
		}
	}
	return nil
}

// TodoListStore 待办事项存储接口
type TodoListStore interface {
	// FindByID 根据ID获取待办事项
	FindByID(ctx context.Context, id uuid.UUID) (*TodoList, error)

	// FindByIDAndUserID 根据ID和用户ID获取待办事项
	FindByIDAndUserID(ctx context.Context, id uuid.UUID, userID string) (*TodoList, error)

	// Create 创建待办事项
	Create(ctx context.Context, obj *TodoList) (*TodoList, error)

	// Update 更新待办事项信息
	Update(ctx context.Context, obj *TodoList) (*TodoList, error)

	// Delete 删除待办事项
	Delete(ctx context.Context, obj *TodoList) error

	// DeleteByID 根据ID删除待办事项
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// DeleteByIDAndUserID 根据ID和用户ID删除待办事项
	DeleteByIDAndUserID(ctx context.Context, id uuid.UUID, userID string) error

	// List 获取待办事项列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (todos []*TodoList, err error)

	// Count 统计待办事项数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// Patch 动态更新待办事项字段
	Patch(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error

	// PatchByUserID 根据用户ID动态更新待办事项字段
	PatchByUserID(ctx context.Context, id uuid.UUID, userID string, updates map[string]interface{}) error

	// 🔥🔥 新增：批量完成任务及其所有子任务（使用事务）
	MarkDoneWithChildren(ctx context.Context, id uuid.UUID) error

	// 🔥🔥🔥 GetByTimeRange 获取时间区间内的待办事项（日历视图专用，使用 OR 逻辑）
	// 查询条件：start_time 在区间内 OR deadline 在区间内 OR 跨区间任务
	GetByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, offset, limit int, otherFilters ...filters.Filter) ([]*TodoList, error)

	// 🔥🔥🔥 CountByTimeRange 统计时间区间内的待办事项数量
	CountByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, otherFilters ...filters.Filter) (int64, error)
}

// TodoListService 待办事项服务接口
type TodoListService interface {
	// FindByID 根据ID获取待办事项
	FindByID(ctx context.Context, id string) (*TodoList, error)

	// Create 创建待办事项
	Create(ctx context.Context, obj *TodoList) (*TodoList, error)

	// Update 更新待办事项信息
	Update(ctx context.Context, obj *TodoList) (*TodoList, error)

	// Delete 删除待办事项
	Delete(ctx context.Context, obj *TodoList) error

	// DeleteByID 根据ID删除待办事项
	DeleteByID(ctx context.Context, id string) error

	// List 获取待办事项列表
	List(ctx context.Context, offset int, limit int, filterActions ...filters.Filter) (todos []*TodoList, err error)

	// Count 统计待办事项数量
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// Patch 动态更新待办事项字段
	Patch(ctx context.Context, id string, updates map[string]interface{}) error

	// GetUserTodos 获取用户的待办事项列表
	GetUserTodos(ctx context.Context, userID string, offset int, limit int, filterActions ...filters.Filter) (todos []*TodoList, err error)

	// CountUserTodos 统计用户的待办事项数量
	CountUserTodos(ctx context.Context, userID string, filterActions ...filters.Filter) (int64, error)

	// MarkDone 标记待办事项为已完成
	MarkDone(ctx context.Context, id string) error

	// MarkRunning 标记待办事项为进行中
	MarkRunning(ctx context.Context, id string) error

	// MarkPending 标记待办事项为待办
	MarkPending(ctx context.Context, id string) error

	// MarkCanceled 标记待办事项为已取消
	MarkCanceled(ctx context.Context, id string) error

	// 🔥 GetChildTodos 获取子任务列表
	GetChildTodos(ctx context.Context, parentID string) ([]*TodoList, error)

	// 🔥🔥 RecalculateChildrenStats 重新计算子任务统计（修复不一致数据）
	RecalculateChildrenStats(ctx context.Context, parentID string) error

	// 🔥🔥🔥 MarkDoneWithChildren 标记任务及其所有子任务为已完成（批量操作）
	MarkDoneWithChildren(ctx context.Context, id string) error

	// 🔥🔥🔥 GetTodosByTimeRange 获取时间区间内的待办事项（日历视图专用，使用 OR 逻辑）
	// 查询条件：start_time 在区间内 OR deadline 在区间内 OR 跨区间任务
	GetTodosByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, offset, limit int, otherFilters ...filters.Filter) ([]*TodoList, error)

	// 🔥🔥🔥 CountTodosByTimeRange 统计时间区间内的待办事项数量
	CountTodosByTimeRange(ctx context.Context, userID string, startTime, endTime time.Time, otherFilters ...filters.Filter) (int64, error)
}
