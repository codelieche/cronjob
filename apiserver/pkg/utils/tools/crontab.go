package tools

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpression 表示一个cron表达式
type CronExpression struct {
	Second  string
	Minute  string
	Hour    string
	Day     string
	Month   string
	Weekday string
	Year    string

	// 解析后的字段
	secondRanges  []int
	minuteRanges  []int
	hourRanges    []int
	dayRanges     []int
	monthRanges   []int
	weekdayRanges []int
	yearRanges    []int

	isValid bool
}

// NewCronExpression 创建并解析一个新的cron表达式
func NewCronExpression(expr string) (*CronExpression, error) {
	cron := &CronExpression{}
	if err := cron.Parse(expr); err != nil {
		return nil, err
	}
	return cron, nil
}

// Parse 解析cron表达式
func (c *CronExpression) Parse(expr string) error {
	// 去除首尾空格
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return errors.New("cron表达式不能为空")
	}

	// 分割表达式
	fields := strings.Fields(expr)
	if len(fields) != 7 {
		return fmt.Errorf("cron表达式格式错误，需要7个字段(秒 分 时 日 月 周 年)，实际有%d个", len(fields))
	}

	// 赋值各个字段
	c.Second = fields[0]
	c.Minute = fields[1]
	c.Hour = fields[2]
	c.Day = fields[3]
	c.Month = fields[4]
	c.Weekday = fields[5]
	c.Year = fields[6]

	// 解析各个字段
	var err error

	// 解析秒 (0-59)
	if c.secondRanges, err = c.parseField(c.Second, 0, 59); err != nil {
		return fmt.Errorf("解析秒字段错误: %v", err)
	}

	// 解析分 (0-59)
	if c.minuteRanges, err = c.parseField(c.Minute, 0, 59); err != nil {
		return fmt.Errorf("解析分字段错误: %v", err)
	}

	// 解析时 (0-23)
	if c.hourRanges, err = c.parseField(c.Hour, 0, 23); err != nil {
		return fmt.Errorf("解析时字段错误: %v", err)
	}

	// 解析日 (1-31)
	if c.dayRanges, err = c.parseField(c.Day, 1, 31); err != nil {
		return fmt.Errorf("解析日字段错误: %v", err)
	}

	// 解析月 (1-12)
	if c.monthRanges, err = c.parseField(c.Month, 1, 12); err != nil {
		return fmt.Errorf("解析月字段错误: %v", err)
	}

	// 解析周 (0-6, 0表示周日)
	if c.weekdayRanges, err = c.parseField(c.Weekday, 0, 6); err != nil {
		return fmt.Errorf("解析周字段错误: %v", err)
	}

	// 解析年 (1970-9999)
	if c.yearRanges, err = c.parseField(c.Year, 1970, 9999); err != nil {
		return fmt.Errorf("解析年字段错误: %v", err)
	}

	c.isValid = true
	return nil
}

// parseField 解析单个字段
func (c *CronExpression) parseField(field string, min, max int) ([]int, error) {
	// 检查是否是通配符
	if field == "*" {
		// 通配符表示全部值
		ranges := make([]int, 0, max-min+1)
		for i := min; i <= max; i++ {
			ranges = append(ranges, i)
		}
		return ranges, nil
	}

	// 检查是否包含范围
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("范围格式错误: %s", field)
		}

		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("起始值不是数字: %s", parts[0])
		}

		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("结束值不是数字: %s", parts[1])
		}

		if start < min || end > max || start > end {
			return nil, fmt.Errorf("范围值超出有效范围(%d-%d): %d-%d", min, max, start, end)
		}

		ranges := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			ranges = append(ranges, i)
		}
		return ranges, nil
	}

	// 检查是否包含步进
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("步进格式错误: %s", field)
		}

		step, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("步长不是数字: %s", parts[1])
		}

		if step <= 0 {
			return nil, fmt.Errorf("步长必须大于0: %d", step)
		}

		var baseRanges []int
		if parts[0] == "*" {
			// 从最小值开始
			baseRanges = make([]int, 0, max-min+1)
			for i := min; i <= max; i++ {
				baseRanges = append(baseRanges, i)
			}
		} else {
			// 解析基础范围
			if strings.Contains(parts[0], "-") {
				subParts := strings.Split(parts[0], "-")
				if len(subParts) != 2 {
					return nil, fmt.Errorf("范围格式错误: %s", parts[0])
				}

				start, err := strconv.Atoi(subParts[0])
				if err != nil {
					return nil, fmt.Errorf("起始值不是数字: %s", subParts[0])
				}

				end, err := strconv.Atoi(subParts[1])
				if err != nil {
					return nil, fmt.Errorf("结束值不是数字: %s", subParts[1])
				}

				if start < min || end > max || start > end {
					return nil, fmt.Errorf("范围值超出有效范围(%d-%d): %d-%d", min, max, start, end)
				}

				baseRanges = make([]int, 0, end-start+1)
				for i := start; i <= end; i++ {
					baseRanges = append(baseRanges, i)
				}
			} else {
				// 单个值
				val, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("值不是数字: %s", parts[0])
				}

				if val < min || val > max {
					return nil, fmt.Errorf("值超出有效范围(%d-%d): %d", min, max, val)
				}

				baseRanges = []int{val}
			}
		}

		// 应用步进
		ranges := make([]int, 0)
		for i := 0; i < len(baseRanges); i += step {
			ranges = append(ranges, baseRanges[i])
		}

		return ranges, nil
	}

	// 检查是否包含列表
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		ranges := make([]int, 0, len(parts))

		for _, part := range parts {
			val, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("列表值不是数字: %s", part)
			}

			if val < min || val > max {
				return nil, fmt.Errorf("列表值超出有效范围(%d-%d): %d", min, max, val)
			}

			ranges = append(ranges, val)
		}

		return ranges, nil
	}

	// 单个值
	val, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("值不是数字: %s", field)
	}

	if val < min || val > max {
		return nil, fmt.Errorf("值超出有效范围(%d-%d): %d", min, max, val)
	}

	return []int{val}, nil
}

// IsValid 检查表达式是否有效
func (c *CronExpression) IsValid() bool {
	return c.isValid
}

// NextExecutionTime 计算下一次执行时间
func (c *CronExpression) NextExecutionTime(fromTime time.Time) (time.Time, error) {
	if !c.isValid {
		return time.Time{}, errors.New("无效的cron表达式")
	}

	// 从下一秒开始
	next := fromTime.Add(1 * time.Second).Truncate(time.Second)

	// 最多尝试10000次，避免死循环
	maxIterations := 10000
	for i := 0; i < maxIterations; i++ {
		// 检查年份
		if !contains(c.yearRanges, next.Year()) {
			// 移动到下一年的第一天00:00:00
			next = time.Date(next.Year()+1, 1, 1, 0, 0, 0, 0, next.Location())
			continue
		}

		// 检查月份
		if !contains(c.monthRanges, int(next.Month())) {
			// 移动到下一个月的第一天00:00:00
			next = time.Date(next.Year(), next.Month()+1, 1, 0, 0, 0, 0, next.Location())
			continue
		}

		// 检查日期
		if !contains(c.dayRanges, next.Day()) {
			// 移动到下一天00:00:00
			next = next.Add(24 * time.Hour).Truncate(24 * time.Hour)
			continue
		}

		// 检查星期
		weekday := int(next.Weekday())
		if !contains(c.weekdayRanges, weekday) {
			// 移动到下一天00:00:00
			next = next.Add(24 * time.Hour).Truncate(24 * time.Hour)
			continue
		}

		// 检查小时
		if !contains(c.hourRanges, next.Hour()) {
			// 移动到下一个小时的00分00秒
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour()+1, 0, 0, 0, next.Location())
			continue
		}

		// 检查分钟
		if !contains(c.minuteRanges, next.Minute()) {
			// 移动到下一分钟的00秒
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), next.Minute()+1, 0, 0, next.Location())
			continue
		}

		// 检查秒
		if !contains(c.secondRanges, next.Second()) {
			// 移动到下一秒
			next = next.Add(1 * time.Second)
			continue
		}

		// 所有字段都匹配，返回这个时间
		return next, nil
	}

	return time.Time{}, errors.New("无法找到下一次执行时间，可能表达式有误")
}

// contains 检查切片是否包含指定的整数
func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// ValidateCronExpression 快速验证cron表达式是否有效
func ValidateCronExpression(expr string) bool {
	cron, err := NewCronExpression(expr)
	return err == nil && cron.IsValid()
}

// GetNextExecutionTime 获取下一次执行时间
func GetNextExecutionTime(expr string, fromTime time.Time) (time.Time, error) {
	cron, err := NewCronExpression(expr)
	if err != nil {
		return time.Time{}, err
	}

	return cron.NextExecutionTime(fromTime)
}
