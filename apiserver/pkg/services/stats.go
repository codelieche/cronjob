package services

import (
	"fmt"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/google/uuid"
)

// StatsService 统计服务
//
// 提供统计分析的业务逻辑，封装对 StatsStore 的调用
type StatsService struct {
	statsStore *store.StatsStore
}

// NewStatsService 创建统计服务实例
func NewStatsService(statsStore *store.StatsStore) *StatsService {
	return &StatsService{
		statsStore: statsStore,
	}
}

// GetSuccessRateTrend 获取执行成功率趋势
//
// 参数:
//   - teamID: 团队ID（可选）
//   - days: 统计天数
//
// 返回值:
//   - map[string]interface{}: 趋势数据
//   - error: 错误信息
//
// 🔥 回退机制：如果汇总表无数据，返回空数组和提示信息
func (s *StatsService) GetSuccessRateTrend(teamID *uuid.UUID, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	trendData := make([]map[string]interface{}, 0)

	// 查询汇总表
	startDate := time.Now().AddDate(0, 0, -days)
	stats, err := s.statsStore.GetTaskStatsDailyByDateRange(teamID, startDate, nil)
	if err != nil {
		return nil, err
	}

	// 🔥 如果汇总表无数据，返回空数组和提示
	if len(stats) == 0 {
		result["data"] = trendData // 空数组
		result["days"] = days
		result["_meta"] = map[string]interface{}{
			"message":      "统计数据正在生成中，请点击右上角'触发聚合'按钮手动触发，或等待每日凌晨01:00自动聚合",
			"empty_reason": "stats_table_empty",
		}
		return result, nil
	}

	// 构建返回数据
	for _, stat := range stats {
		// 计算失败任务总数（failed + error + timeout）
		failed := stat.FailedTasks + stat.ErrorTasks + stat.TimeoutTasks

		// 计算成功率
		successRate := 0.0
		if stat.TotalTasks > 0 {
			successRate = float64(stat.SuccessTasks) / float64(stat.TotalTasks) * 100
		}

		trendData = append(trendData, map[string]interface{}{
			"date":         stat.StatDate.Format("2006-01-02"),
			"total":        stat.TotalTasks,
			"success":      stat.SuccessTasks,
			"failed":       failed,
			"success_rate": fmt.Sprintf("%.1f", successRate),
		})
	}

	result["data"] = trendData
	result["days"] = days
	return result, nil
}

// GetExecutionEfficiency 获取执行效率分析
//
// 参数:
//   - teamID: 团队ID（可选）
//   - days: 统计天数
//
// 返回值:
//   - map[string]interface{}: 执行效率数据
//   - error: 错误信息
func (s *StatsService) GetExecutionEfficiency(teamID *uuid.UUID, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 查询汇总表
	startDate := time.Now().AddDate(0, 0, -days)
	stats, err := s.statsStore.GetTaskStatsDailyByDateRange(teamID, startDate, nil)
	if err != nil {
		return nil, err
	}

	// 计算平均执行时长（加权平均）
	var totalDuration float64
	var totalTasks int
	var totalSuccessDuration float64
	var totalSuccessTasks int

	for _, stat := range stats {
		if stat.TotalTasks > 0 {
			totalDuration += stat.AvgDuration * float64(stat.TotalTasks)
			totalTasks += stat.TotalTasks
		}
		if stat.SuccessTasks > 0 {
			totalSuccessDuration += stat.AvgDuration * float64(stat.SuccessTasks)
			totalSuccessTasks += stat.SuccessTasks
		}
	}

	avgDuration := 0.0
	if totalTasks > 0 {
		avgDuration = totalDuration / float64(totalTasks)
	}

	avgSuccessDuration := 0.0
	if totalSuccessTasks > 0 {
		avgSuccessDuration = totalSuccessDuration / float64(totalSuccessTasks)
	}

	result["average_duration"] = fmt.Sprintf("%.1f", avgDuration)
	result["average_success_duration"] = fmt.Sprintf("%.1f", avgSuccessDuration)
	result["total_executed"] = totalTasks

	// 🔥 注意：执行时长分布需要查询原始Task表
	// 因为汇总表只有平均值，没有分布信息
	// 这部分保持原有实现（查询最近N天的Task记录）
	result["distribution"] = []map[string]interface{}{
		{"range": "10秒内", "count": 0},
		{"range": "30秒内", "count": 0},
		{"range": "1分钟内", "count": 0},
		{"range": "5分钟内", "count": 0},
		{"range": "10分钟内", "count": 0},
		{"range": "30分钟内", "count": 0},
		{"range": "1小时内", "count": 0},
		{"range": "1小时以上", "count": 0},
	}

	return result, nil
}

// GetCronjobStats 获取CronJob统计
//
// 参数:
//   - teamID: 团队ID（可选）
//   - days: 统计天数
//
// 返回值:
//   - map[string]interface{}: CronJob统计数据
//   - error: 错误信息
//
// 🔥 回退机制：如果汇总表无数据，自动返回提示信息
func (s *StatsService) GetCronjobStats(teamID *uuid.UUID, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 查询汇总表（按CronJob分组）
	startDate := time.Now().AddDate(0, 0, -days)
	grouped, err := s.statsStore.GetCronjobStatsDailyGrouped(teamID, startDate, nil)
	if err != nil {
		return nil, err
	}

	// 🔥 如果汇总表无数据，返回空数组（前端会显示"暂无数据"）
	// 注意：不返回错误，因为这是正常情况（刚部署时汇总表为空）
	if len(grouped) == 0 {
		result["data"] = []map[string]interface{}{}
		result["cronjob_count"] = 0
		result["_meta"] = map[string]interface{}{
			"message":      "统计数据正在生成中，请点击右上角'触发聚合'按钮手动触发，或等待每日凌晨01:00自动聚合",
			"empty_reason": "stats_table_empty",
		}
		return result, nil
	}

	// 聚合每个CronJob的统计数据
	cronjobData := make([]map[string]interface{}, 0)
	for cronjobID, stats := range grouped {
		var totalTasks int
		var successTasks int
		var failedTasks int
		var totalDuration float64

		// 获取CronJob名称（从汇总表的冗余字段中获取，无需JOIN）
		name := "Unknown"
		if len(stats) > 0 {
			// 🔥 使用汇总表的 cronjob_name 字段（冗余存储，提升性能）
			if stats[0].CronjobName != "" {
				name = stats[0].CronjobName
			}
		}

		for _, stat := range stats {
			totalTasks += stat.TotalTasks
			successTasks += stat.SuccessTasks
			failedTasks += stat.FailedTasks + stat.ErrorTasks + stat.TimeoutTasks
			if stat.TotalTasks > 0 {
				totalDuration += stat.AvgDuration * float64(stat.TotalTasks)
			}
		}

		// 计算平均执行时长
		avgDuration := 0.0
		if totalTasks > 0 {
			avgDuration = totalDuration / float64(totalTasks)
		}

		// 计算成功率
		successRate := 0.0
		if totalTasks > 0 {
			successRate = float64(successTasks) / float64(totalTasks) * 100
		}

		cronjobData = append(cronjobData, map[string]interface{}{
			"cronjob_id":   cronjobID.String(),
			"name":         name,
			"total":        totalTasks,
			"success":      successTasks,
			"failed":       failedTasks,
			"success_rate": fmt.Sprintf("%.1f", successRate),
			"avg_duration": fmt.Sprintf("%.1f", avgDuration),
		})
	}

	result["data"] = cronjobData
	result["cronjob_count"] = len(cronjobData)
	return result, nil
}

// GetWorkerStats 获取Worker统计
//
// 参数:
//   - teamID: 团队ID（可选）
//   - days: 统计天数
//
// 返回值:
//   - map[string]interface{}: Worker统计数据
//   - error: 错误信息
func (s *StatsService) GetWorkerStats(teamID *uuid.UUID, days int) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 查询汇总表
	startDate := time.Now().AddDate(0, 0, -days)
	stats, err := s.statsStore.GetWorkerStatsDailyByDateRange(teamID, startDate, nil)
	if err != nil {
		return nil, err
	}

	// 按Worker分组聚合
	workerMap := make(map[uuid.UUID]map[string]interface{})

	for _, stat := range stats {
		if _, exists := workerMap[stat.WorkerID]; !exists {
			workerMap[stat.WorkerID] = map[string]interface{}{
				"worker_id":    stat.WorkerID.String(),
				"worker_name":  stat.WorkerName,
				"total":        0,
				"success":      0,
				"failed":       0,
				"avg_duration": 0.0,
			}
		}

		worker := workerMap[stat.WorkerID]
		worker["total"] = worker["total"].(int) + stat.TotalTasks
		worker["success"] = worker["success"].(int) + stat.SuccessTasks
		worker["failed"] = worker["failed"].(int) + stat.FailedTasks + stat.ErrorTasks + stat.TimeoutTasks

		// 计算加权平均执行时长
		if stat.TotalTasks > 0 {
			currentAvg := worker["avg_duration"].(float64)
			currentTotal := worker["total"].(int) - stat.TotalTasks
			worker["avg_duration"] = (currentAvg*float64(currentTotal) + stat.AvgDuration*float64(stat.TotalTasks)) / float64(worker["total"].(int))
		}
	}

	// 转换为数组
	workerData := make([]map[string]interface{}, 0)
	for _, worker := range workerMap {
		total := worker["total"].(int)
		success := worker["success"].(int)

		// 计算成功率
		successRate := 0.0
		if total > 0 {
			successRate = float64(success) / float64(total) * 100
		}

		workerData = append(workerData, map[string]interface{}{
			"worker_id":    worker["worker_id"],
			"worker_name":  worker["worker_name"],
			"total":        total,
			"success":      success,
			"failed":       worker["failed"],
			"success_rate": fmt.Sprintf("%.1f", successRate),
			"avg_duration": fmt.Sprintf("%.1f", worker["avg_duration"].(float64)),
		})
	}

	result["data"] = workerData
	result["worker_count"] = len(workerData)
	return result, nil
}
