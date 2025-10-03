package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/shard"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/filters"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 🔥🔥 Context中的优化信息键
type contextKey string

const (
	// TaskLogOptimizationKey 用于在context中传递TaskLog查询优化信息
	TaskLogOptimizationKey contextKey = "tasklog_optimization"
)

// TaskLogOptimization TaskLog查询优化信息
type TaskLogOptimization struct {
	// CreatedAt 精确的创建时间（优先级最高）
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// StartTime 开始时间范围
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间范围
	EndTime *time.Time `json:"end_time,omitempty"`
}

// WithTaskLogOptimization 在context中设置TaskLog优化信息
func WithTaskLogOptimization(ctx context.Context, opt *TaskLogOptimization) context.Context {
	return context.WithValue(ctx, TaskLogOptimizationKey, opt)
}

// GetTaskLogOptimization 从context中获取TaskLog优化信息
func GetTaskLogOptimization(ctx context.Context) (*TaskLogOptimization, bool) {
	opt, ok := ctx.Value(TaskLogOptimizationKey).(*TaskLogOptimization)
	return opt, ok
}

// TaskLogShardStore 分片感知的TaskLog存储接口
type TaskLogShardStore interface {
	// 基础CRUD操作
	Create(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error)
	FindByTaskID(ctx context.Context, taskID string) (*core.TaskLog, error)
	Update(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error)
	DeleteByTaskID(ctx context.Context, taskID string) error

	// 🔥🔥 智能查询方法 - 自动从context中获取优化信息
	FindByTaskIDSmart(ctx context.Context, taskID string) (*core.TaskLog, error)
	UpdateSmart(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error)
	DeleteByTaskIDSmart(ctx context.Context, taskID string) error

	// 列表查询（支持分片）
	List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error)
	Count(ctx context.Context, filterActions ...filters.Filter) (int64, error)

	// 权限控制查询
	ListByTeams(ctx context.Context, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error)
	CountByTeams(ctx context.Context, teamIDs []string, filterActions ...filters.Filter) (int64, error)
}

// taskLogShardStore 分片TaskLog存储实现
type taskLogShardStore struct {
	db           *gorm.DB
	shardManager *shard.ShardManager
}

// NewTaskLogShardStore 创建分片TaskLog存储
func NewTaskLogShardStore(db *gorm.DB, shardManager *shard.ShardManager) TaskLogShardStore {
	return &taskLogShardStore{
		db:           db,
		shardManager: shardManager,
	}
}

// Create 创建TaskLog - 根据创建时间写入对应分片表
func (s *taskLogShardStore) Create(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 1. 设置时间戳
	now := time.Now()
	taskLog.CreatedAt = now
	taskLog.UpdatedAt = now

	// 2. 确定分片表名
	tableName := s.shardManager.GetTableName(taskLog.CreatedAt)

	// 3. 确保分片表存在
	if err := s.shardManager.EnsureTableExistsByName(tableName); err != nil {
		return nil, fmt.Errorf("确保分片表存在失败: %w", err)
	}

	// 4. 写入分片表
	result := s.db.WithContext(ctx).Table(tableName).Create(taskLog)
	if result.Error != nil {
		return nil, fmt.Errorf("创建TaskLog失败: %w", result.Error)
	}

	logger.Debug("成功创建TaskLog",
		zap.String("task_id", taskLog.TaskID.String()),
		zap.String("table", tableName))

	return taskLog, nil
}

// FindByTaskID 根据TaskID查找TaskLog - 需要跨分片查询
func (s *taskLogShardStore) FindByTaskID(ctx context.Context, taskID string) (*core.TaskLog, error) {
	return s.FindByTaskIDWithTimeRange(ctx, taskID, nil, nil, nil)
}

// FindByTaskIDWithTimeRange 根据TaskID和时间信息查找TaskLog
// 🔥 性能优化：支持精确时间或时间范围过滤，避免查询所有分片表
// createdAt: 精确的创建时间，如果提供则直接定位到唯一分片表（性能最优）
// startTime/endTime: 时间范围，如果createdAt为nil则使用范围查询
func (s *taskLogShardStore) FindByTaskIDWithTimeRange(ctx context.Context, taskID string, createdAt *time.Time, startTime, endTime *time.Time) (*core.TaskLog, error) {
	// 1. 解析TaskID
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("无效的TaskID格式: %w", err)
	}

	// 2. 🔥🔥 优先使用精确时间定位（性能最优）
	if createdAt != nil {
		tableName := s.shardManager.GetTableName(*createdAt)

		logger.Debug("使用精确时间定位分片表查询",
			zap.String("task_id", taskID),
			zap.Time("created_at", *createdAt),
			zap.String("table_name", tableName))

		// 直接在精确的分片表中查询（只查询一个表！）
		var taskLog core.TaskLog
		result := s.db.WithContext(ctx).Table(tableName).
			Where("task_id = ?", taskUUID).
			First(&taskLog)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil, core.ErrNotFound
			}
			return nil, fmt.Errorf("查询TaskLog失败: %w", result.Error)
		}

		logger.Debug("成功从精确分片表查询到TaskLog",
			zap.String("task_id", taskID),
			zap.String("table_name", tableName))

		return &taskLog, nil
	}

	// 3. 🔥 使用时间范围查询（次优性能）
	var tables []string
	if startTime != nil && endTime != nil {
		// 使用指定的时间范围，只查询相关分片表
		tables = s.shardManager.GetTablesInRange(startTime, endTime)
		logger.Debug("使用时间范围优化分片表查询",
			zap.String("task_id", taskID),
			zap.Time("start_time", *startTime),
			zap.Time("end_time", *endTime),
			zap.Int("table_count", len(tables)))
	} else {
		// 没有时间信息时，默认查询最近3个月（减少默认范围）
		now := time.Now()
		defaultStart := now.AddDate(0, -3, 0)
		tables = s.shardManager.GetTablesInRange(&defaultStart, &now)
		logger.Debug("使用默认时间范围查询分片表",
			zap.String("task_id", taskID),
			zap.Int("table_count", len(tables)))
	}

	// 4. 并发查询选定的分片表
	return s.findTaskLogInTables(ctx, taskUUID, tables)
}

// 🔥🔥 智能查询方法 - 自动从context获取优化信息
// FindByTaskIDSmart 智能查询TaskLog，自动从context中获取优化信息
func (s *taskLogShardStore) FindByTaskIDSmart(ctx context.Context, taskID string) (*core.TaskLog, error) {
	// 1. 尝试从context中获取优化信息
	if opt, ok := GetTaskLogOptimization(ctx); ok {
		logger.Debug("使用context中的优化信息进行智能查询",
			zap.String("task_id", taskID),
			zap.Any("optimization", opt))

		return s.FindByTaskIDWithTimeRange(ctx, taskID, opt.CreatedAt, opt.StartTime, opt.EndTime)
	}

	// 2. 降级到普通查询
	return s.FindByTaskID(ctx, taskID)
}

// UpdateSmart 智能更新TaskLog，优先使用context中的优化信息
func (s *taskLogShardStore) UpdateSmart(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 1. 尝试从context中获取优化信息进行快速查询
	if opt, ok := GetTaskLogOptimization(ctx); ok && opt.CreatedAt != nil {
		// 🔥🔥 使用精确时间直接定位分片表进行更新
		tableName := s.shardManager.GetTableName(*opt.CreatedAt)

		logger.Debug("使用精确时间进行智能更新",
			zap.String("task_id", taskLog.TaskID.String()),
			zap.Time("created_at", *opt.CreatedAt),
			zap.String("table_name", tableName))

		// 更新时间戳
		taskLog.UpdatedAt = time.Now()

		// 直接在精确的分片表中更新
		result := s.db.WithContext(ctx).Table(tableName).
			Where("task_id = ?", taskLog.TaskID).
			Updates(taskLog)

		if result.Error != nil {
			return nil, fmt.Errorf("更新TaskLog失败: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return nil, core.ErrNotFound
		}

		logger.Debug("成功智能更新TaskLog",
			zap.String("task_id", taskLog.TaskID.String()),
			zap.String("table", tableName))

		return taskLog, nil
	}

	// 2. 降级到普通更新（需要先查询确定分片表）
	return s.Update(ctx, taskLog)
}

// DeleteByTaskIDSmart 智能删除TaskLog，优先使用context中的优化信息
func (s *taskLogShardStore) DeleteByTaskIDSmart(ctx context.Context, taskID string) error {
	// 1. 尝试从context中获取优化信息进行快速删除
	if opt, ok := GetTaskLogOptimization(ctx); ok && opt.CreatedAt != nil {
		// 🔥🔥 使用精确时间直接定位分片表进行删除
		tableName := s.shardManager.GetTableName(*opt.CreatedAt)

		logger.Debug("使用精确时间进行智能删除",
			zap.String("task_id", taskID),
			zap.Time("created_at", *opt.CreatedAt),
			zap.String("table_name", tableName))

		// 直接在精确的分片表中删除
		result := s.db.WithContext(ctx).Table(tableName).
			Where("task_id = ?", taskID).
			Delete(&core.TaskLog{})

		if result.Error != nil {
			return fmt.Errorf("删除TaskLog失败: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return core.ErrNotFound
		}

		logger.Debug("成功智能删除TaskLog",
			zap.String("task_id", taskID),
			zap.String("table", tableName))

		return nil
	}

	// 2. 降级到普通删除（需要先查询确定分片表）
	return s.DeleteByTaskID(ctx, taskID)
}

// Update 更新TaskLog - 根据TaskID找到对应分片表后更新
func (s *taskLogShardStore) Update(ctx context.Context, taskLog *core.TaskLog) (*core.TaskLog, error) {
	// 1. 先查找现有记录确定分片表
	existing, err := s.FindByTaskID(ctx, taskLog.TaskID.String())
	if err != nil {
		return nil, err
	}

	// 2. 确定分片表名（使用现有记录的创建时间）
	tableName := s.shardManager.GetTableName(existing.CreatedAt)

	// 3. 更新时间戳
	taskLog.UpdatedAt = time.Now()

	// 4. 更新分片表中的记录
	result := s.db.WithContext(ctx).Table(tableName).
		Where("task_id = ?", taskLog.TaskID).
		Updates(taskLog)

	if result.Error != nil {
		return nil, fmt.Errorf("更新TaskLog失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, core.ErrNotFound
	}

	logger.Debug("成功更新TaskLog",
		zap.String("task_id", taskLog.TaskID.String()),
		zap.String("table", tableName))

	return taskLog, nil
}

// DeleteByTaskID 根据TaskID删除TaskLog
func (s *taskLogShardStore) DeleteByTaskID(ctx context.Context, taskID string) error {
	// 1. 先查找现有记录确定分片表
	existing, err := s.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}

	// 2. 确定分片表名
	tableName := s.shardManager.GetTableName(existing.CreatedAt)

	// 3. 删除记录
	result := s.db.WithContext(ctx).Table(tableName).
		Where("task_id = ?", taskID).
		Delete(&core.TaskLog{})

	if result.Error != nil {
		return fmt.Errorf("删除TaskLog失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return core.ErrNotFound
	}

	logger.Debug("成功删除TaskLog",
		zap.String("task_id", taskID),
		zap.String("table", tableName))

	return nil
}

// List 列表查询 - 跨分片查询
func (s *taskLogShardStore) List(ctx context.Context, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	// 1. 从过滤条件中提取时间范围
	timeRange := s.extractTimeRangeFromFilters(filterActions)

	// 2. 获取需要查询的分片表
	tables := s.shardManager.GetTablesInRange(timeRange.Start, timeRange.End)

	if len(tables) == 0 {
		return []*core.TaskLog{}, nil
	}

	// 3. 并发查询所有分片表
	return s.queryMultipleShards(ctx, tables, offset, limit, filterActions...)
}

// Count 计数查询 - 跨分片查询
func (s *taskLogShardStore) Count(ctx context.Context, filterActions ...filters.Filter) (int64, error) {
	// 1. 从过滤条件中提取时间范围
	timeRange := s.extractTimeRangeFromFilters(filterActions)

	// 2. 获取需要查询的分片表
	tables := s.shardManager.GetTablesInRange(timeRange.Start, timeRange.End)

	if len(tables) == 0 {
		return 0, nil
	}

	// 3. 并发查询所有分片表的计数
	return s.countMultipleShards(ctx, tables, filterActions...)
}

// ListByTeams 根据团队列表查询TaskLog
func (s *taskLogShardStore) ListByTeams(ctx context.Context, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	if len(teamIDs) == 0 {
		return []*core.TaskLog{}, nil
	}

	// 1. 从过滤条件中提取时间范围
	timeRange := s.extractTimeRangeFromFilters(filterActions)

	// 2. 获取需要查询的分片表
	tables := s.shardManager.GetTablesInRange(timeRange.Start, timeRange.End)

	if len(tables) == 0 {
		return []*core.TaskLog{}, nil
	}

	// 3. 并发查询所有分片表（通过JOIN task表过滤团队）
	return s.queryMultipleShardsWithTeamFilter(ctx, tables, teamIDs, offset, limit, filterActions...)
}

// CountByTeams 根据团队列表计数TaskLog
func (s *taskLogShardStore) CountByTeams(ctx context.Context, teamIDs []string, filterActions ...filters.Filter) (int64, error) {
	if len(teamIDs) == 0 {
		return 0, nil
	}

	// 1. 从过滤条件中提取时间范围
	timeRange := s.extractTimeRangeFromFilters(filterActions)

	// 2. 获取需要查询的分片表
	tables := s.shardManager.GetTablesInRange(timeRange.Start, timeRange.End)

	if len(tables) == 0 {
		return 0, nil
	}

	// 3. 并发查询所有分片表的计数（通过JOIN task表过滤团队）
	return s.countMultipleShardsWithTeamFilter(ctx, tables, teamIDs, filterActions...)
}

// TimeRange 时间范围
type TimeRange struct {
	Start *time.Time
	End   *time.Time
}

// extractTimeRangeFromFilters 从过滤条件中提取时间范围
func (s *taskLogShardStore) extractTimeRangeFromFilters(filterActions []filters.Filter) *TimeRange {
	var startTime, endTime *time.Time

	for _, filter := range filterActions {
		if filterOpt, ok := filter.(*filters.FilterOption); ok {
			if filterOpt.Column == "created_at" {
				switch filterOpt.Op {
				case filters.FILTER_GTE:
					if t := s.parseTimeValue(filterOpt.Value); t != nil {
						startTime = t
					}
				case filters.FILTER_LTE:
					if t := s.parseTimeValue(filterOpt.Value); t != nil {
						endTime = t
					}
				}
			}
		}
	}

	return &TimeRange{
		Start: startTime,
		End:   endTime,
	}
}

// parseTimeValue 解析时间值
func (s *taskLogShardStore) parseTimeValue(value interface{}) *time.Time {
	switch v := value.(type) {
	case time.Time:
		return &v
	case string:
		if t, err := time.Parse("2006-01-02", v); err == nil {
			return &t
		}
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			return &t
		}
	}
	return nil
}

// findTaskLogInTables 在多个分片表中查找TaskLog
func (s *taskLogShardStore) findTaskLogInTables(ctx context.Context, taskID uuid.UUID, tables []string) (*core.TaskLog, error) {
	type result struct {
		taskLog *core.TaskLog
		err     error
	}

	results := make(chan result, len(tables))
	var wg sync.WaitGroup

	// 并发查询所有分片表
	for _, tableName := range tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()

			var taskLog core.TaskLog
			err := s.db.WithContext(ctx).Table(table).
				Where("task_id = ?", taskID).
				First(&taskLog).Error

			if err != nil && err != gorm.ErrRecordNotFound {
				results <- result{nil, err}
				return
			}

			if err == nil {
				results <- result{&taskLog, nil}
				return
			}

			// 记录未找到，不发送结果
		}(tableName)
	}

	// 等待所有查询完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 处理结果
	for res := range results {
		if res.err != nil {
			logger.Error("查询分片表失败", zap.Error(res.err))
			continue
		}
		if res.taskLog != nil {
			return res.taskLog, nil
		}
	}

	return nil, core.ErrNotFound
}

// queryMultipleShards 查询多个分片表
func (s *taskLogShardStore) queryMultipleShards(ctx context.Context, tables []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	type shardResult struct {
		tableName string
		taskLogs  []*core.TaskLog
		err       error
	}

	results := make(chan shardResult, len(tables))
	var wg sync.WaitGroup

	// 并发查询每个分片表
	for _, tableName := range tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()

			taskLogs, err := s.queryShardTable(ctx, table, 0, 0, filterActions...)
			results <- shardResult{
				tableName: table,
				taskLogs:  taskLogs,
				err:       err,
			}
		}(tableName)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 聚合结果
	var allTaskLogs []*core.TaskLog
	for res := range results {
		if res.err != nil {
			logger.Error("查询分片表失败", zap.String("table", res.tableName), zap.Error(res.err))
			continue
		}
		allTaskLogs = append(allTaskLogs, res.taskLogs...)
	}

	// 跨分片排序
	s.sortTaskLogs(allTaskLogs)

	// 跨分片分页
	return s.paginateTaskLogs(allTaskLogs, offset, limit), nil
}

// queryShardTable 查询单个分片表
func (s *taskLogShardStore) queryShardTable(ctx context.Context, tableName string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	query := s.db.WithContext(ctx).Table(tableName)

	// 应用过滤条件
	for _, filter := range filterActions {
		query = filter.Filter(query)
	}

	// 排序
	query = query.Order("created_at DESC")

	// 分页（如果指定）
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var taskLogs []*core.TaskLog
	if err := query.Find(&taskLogs).Error; err != nil {
		return nil, fmt.Errorf("查询分片表 %s 失败: %w", tableName, err)
	}

	return taskLogs, nil
}

// queryMultipleShardsWithTeamFilter 查询多个分片表（带团队过滤）
func (s *taskLogShardStore) queryMultipleShardsWithTeamFilter(ctx context.Context, tables []string, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	type shardResult struct {
		tableName string
		taskLogs  []*core.TaskLog
		err       error
	}

	results := make(chan shardResult, len(tables))
	var wg sync.WaitGroup

	// 并发查询每个分片表
	for _, tableName := range tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()

			taskLogs, err := s.queryShardTableWithTeamFilter(ctx, table, teamIDs, 0, 0, filterActions...)
			results <- shardResult{
				tableName: table,
				taskLogs:  taskLogs,
				err:       err,
			}
		}(tableName)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 聚合结果
	var allTaskLogs []*core.TaskLog
	for res := range results {
		if res.err != nil {
			logger.Error("查询分片表失败", zap.String("table", res.tableName), zap.Error(res.err))
			continue
		}
		allTaskLogs = append(allTaskLogs, res.taskLogs...)
	}

	// 跨分片排序
	s.sortTaskLogs(allTaskLogs)

	// 跨分片分页
	return s.paginateTaskLogs(allTaskLogs, offset, limit), nil
}

// queryShardTableWithTeamFilter 查询单个分片表（带团队过滤）
func (s *taskLogShardStore) queryShardTableWithTeamFilter(ctx context.Context, tableName string, teamIDs []string, offset, limit int, filterActions ...filters.Filter) ([]*core.TaskLog, error) {
	// 🔥 关键优化：使用JOIN查询，避免大量IN操作
	query := s.db.WithContext(ctx).
		Table(fmt.Sprintf("%s tl", tableName)).
		Select("tl.*").
		Joins("INNER JOIN tasks t ON tl.task_id = t.id").
		Where("t.team_id IN (?)", teamIDs)

	// 应用其他过滤条件（注意表别名）
	for _, filter := range filterActions {
		if filterOpt, ok := filter.(*filters.FilterOption); ok {
			// 为TaskLog字段添加表别名
			column := filterOpt.Column
			if !strings.Contains(column, ".") {
				column = "tl." + column
			}

			// 创建新的过滤器选项
			newFilter := &filters.FilterOption{
				Column: column,
				Value:  filterOpt.Value,
				Op:     filterOpt.Op,
			}
			query = newFilter.Filter(query)
		}
	}

	// 排序
	query = query.Order("tl.created_at DESC")

	// 分页（如果指定）
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var taskLogs []*core.TaskLog
	if err := query.Find(&taskLogs).Error; err != nil {
		return nil, fmt.Errorf("查询分片表 %s 失败: %w", tableName, err)
	}

	return taskLogs, nil
}

// countMultipleShards 计数多个分片表
func (s *taskLogShardStore) countMultipleShards(ctx context.Context, tables []string, filterActions ...filters.Filter) (int64, error) {
	type countResult struct {
		tableName string
		count     int64
		err       error
	}

	results := make(chan countResult, len(tables))
	var wg sync.WaitGroup

	// 并发查询每个分片表的计数
	for _, tableName := range tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()

			count, err := s.countShardTable(ctx, table, filterActions...)
			results <- countResult{
				tableName: table,
				count:     count,
				err:       err,
			}
		}(tableName)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 聚合计数
	var totalCount int64
	for res := range results {
		if res.err != nil {
			logger.Error("计数分片表失败", zap.String("table", res.tableName), zap.Error(res.err))
			continue
		}
		totalCount += res.count
	}

	return totalCount, nil
}

// countShardTable 计数单个分片表
func (s *taskLogShardStore) countShardTable(ctx context.Context, tableName string, filterActions ...filters.Filter) (int64, error) {
	query := s.db.WithContext(ctx).Table(tableName)

	// 应用过滤条件
	for _, filter := range filterActions {
		query = filter.Filter(query)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("计数分片表 %s 失败: %w", tableName, err)
	}

	return count, nil
}

// countMultipleShardsWithTeamFilter 计数多个分片表（带团队过滤）
func (s *taskLogShardStore) countMultipleShardsWithTeamFilter(ctx context.Context, tables []string, teamIDs []string, filterActions ...filters.Filter) (int64, error) {
	type countResult struct {
		tableName string
		count     int64
		err       error
	}

	results := make(chan countResult, len(tables))
	var wg sync.WaitGroup

	// 并发查询每个分片表的计数
	for _, tableName := range tables {
		wg.Add(1)
		go func(table string) {
			defer wg.Done()

			count, err := s.countShardTableWithTeamFilter(ctx, table, teamIDs, filterActions...)
			results <- countResult{
				tableName: table,
				count:     count,
				err:       err,
			}
		}(tableName)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 聚合计数
	var totalCount int64
	for res := range results {
		if res.err != nil {
			logger.Error("计数分片表失败", zap.String("table", res.tableName), zap.Error(res.err))
			continue
		}
		totalCount += res.count
	}

	return totalCount, nil
}

// countShardTableWithTeamFilter 计数单个分片表（带团队过滤）
func (s *taskLogShardStore) countShardTableWithTeamFilter(ctx context.Context, tableName string, teamIDs []string, filterActions ...filters.Filter) (int64, error) {
	// 使用JOIN查询计数
	query := s.db.WithContext(ctx).
		Table(fmt.Sprintf("%s tl", tableName)).
		Joins("INNER JOIN tasks t ON tl.task_id = t.id").
		Where("t.team_id IN ?", teamIDs)

	// 应用其他过滤条件
	for _, filter := range filterActions {
		if filterOpt, ok := filter.(*filters.FilterOption); ok {
			column := filterOpt.Column
			if !strings.Contains(column, ".") {
				column = "tl." + column
			}

			newFilter := &filters.FilterOption{
				Column: column,
				Value:  filterOpt.Value,
				Op:     filterOpt.Op,
			}
			query = newFilter.Filter(query)
		}
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("计数分片表 %s 失败: %w", tableName, err)
	}

	return count, nil
}

// sortTaskLogs 对TaskLog列表进行排序
func (s *taskLogShardStore) sortTaskLogs(taskLogs []*core.TaskLog) {
	// 按创建时间降序排序
	for i := 0; i < len(taskLogs)-1; i++ {
		for j := i + 1; j < len(taskLogs); j++ {
			if taskLogs[i].CreatedAt.Before(taskLogs[j].CreatedAt) {
				taskLogs[i], taskLogs[j] = taskLogs[j], taskLogs[i]
			}
		}
	}
}

// paginateTaskLogs 对TaskLog列表进行分页
func (s *taskLogShardStore) paginateTaskLogs(taskLogs []*core.TaskLog, offset, limit int) []*core.TaskLog {
	if offset >= len(taskLogs) {
		return []*core.TaskLog{}
	}

	end := offset + limit
	if end > len(taskLogs) {
		end = len(taskLogs)
	}

	return taskLogs[offset:end]
}
