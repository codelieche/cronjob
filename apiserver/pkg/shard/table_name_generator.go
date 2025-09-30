package shard

import (
	"fmt"
	"strings"
	"time"
)

// TableNameGenerator 表名生成器接口
type TableNameGenerator interface {
	Generate(timestamp time.Time) string
	Parse(tableName string) (time.Time, error)
	GetNextMonthTable() string
}

// monthlyTableNameGenerator 按月分片的表名生成器
type monthlyTableNameGenerator struct {
	prefix string
}

// NewTableNameGenerator 创建表名生成器
func NewTableNameGenerator(prefix string) TableNameGenerator {
	return &monthlyTableNameGenerator{
		prefix: prefix,
	}
}

// Generate 根据时间戳生成表名
// 格式：{prefix}_{YYYYMM}
// 例如：task_logs_202509
func (g *monthlyTableNameGenerator) Generate(timestamp time.Time) string {
	return fmt.Sprintf("%s_%s", g.prefix, timestamp.Format("200601"))
}

// Parse 从表名解析出时间
func (g *monthlyTableNameGenerator) Parse(tableName string) (time.Time, error) {
	// 从 "task_logs_202509" 解析出 "202509"
	parts := strings.Split(tableName, "_")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid table name format: %s", tableName)
	}

	// 取最后一部分作为日期
	dateStr := parts[len(parts)-1]
	if len(dateStr) != 6 {
		return time.Time{}, fmt.Errorf("invalid date format in table name: %s", tableName)
	}

	// 解析YYYYMM格式
	parsedTime, err := time.Parse("200601", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse date from table name %s: %w", tableName, err)
	}

	return parsedTime, nil
}

// GetNextMonthTable 获取下月表名
func (g *monthlyTableNameGenerator) GetNextMonthTable() string {
	nextMonth := time.Now().AddDate(0, 1, 0)
	return g.Generate(nextMonth)
}
