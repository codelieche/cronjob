package tools

import (
	"testing"
	"time"
)

func TestNewCronExpression(t *testing.T) {
	// 测试有效表达式
	expr := "0 0 12 * * 1-5 *"
	cron, err := NewCronExpression(expr)
	if err != nil {
		t.Errorf("解析有效表达式失败: %v", err)
	}
	if !cron.IsValid() {
		t.Errorf("表达式应该有效，但被标记为无效")
	}

	// 测试无效表达式 - 字段数量不正确
	expr = "0 0 12 * * 1-5"
	_, err = NewCronExpression(expr)
	if err == nil {
		t.Errorf("解析无效表达式(字段数量)应该失败，但成功了")
	}

	// 测试无效表达式 - 值超出范围
	expr = "60 0 12 * * 1-5 *"
	_, err = NewCronExpression(expr)
	if err == nil {
		t.Errorf("解析无效表达式(值超出范围)应该失败，但成功了")
	}

	// 测试无效表达式 - 格式错误
	expr = "* * * * * * invalid"
	_, err = NewCronExpression(expr)
	if err == nil {
		t.Errorf("解析无效表达式(格式错误)应该失败，但成功了")
	}
}

func TestValidateCronExpression(t *testing.T) {
	// 测试有效表达式
	expr := "0 0 12 * * 1-5 *"
	if !ValidateCronExpression(expr) {
		t.Errorf("有效表达式应该被验证为有效，但被标记为无效")
	}

	// 测试无效表达式
	expr = "0 0 12 * * 1-5"
	if ValidateCronExpression(expr) {
		t.Errorf("无效表达式应该被验证为无效，但被标记为有效")
	}
}

func TestCronExpression_NextExecutionTime(t *testing.T) {
	// 测试每秒执行
	expr := "* * * * * * *"
	cron, err := NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	next, err := cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	// 应该是下一秒
	expected := now.Add(1 * time.Second)
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}

	// 测试每分钟执行
	expr = "0 * * * * * *"
	cron, err = NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now = time.Date(2023, 1, 1, 10, 15, 30, 0, time.UTC)
	next, err = cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	expected = time.Date(2023, 1, 1, 10, 16, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}

	// 测试每小时执行
	expr = "0 0 * * * * *"
	cron, err = NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now = time.Date(2023, 1, 1, 10, 15, 30, 0, time.UTC)
	next, err = cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	expected = time.Date(2023, 1, 1, 11, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}

	// 测试工作日中午12点执行
	expr = "0 0 12 * * 1-5 *"
	cron, err = NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	// 模拟周六
	now = time.Date(2023, 1, 7, 10, 15, 30, 0, time.UTC) // 周六 (周日是0，周六是6)
	next, err = cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	// 应该是下周一的12点
	expected = time.Date(2023, 1, 9, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}

	// 测试步进表达式
	expr = "0 */5 * * * * *" // 每5分钟执行一次
	cron, err = NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now = time.Date(2023, 1, 1, 10, 15, 30, 0, time.UTC)
	next, err = cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	expected = time.Date(2023, 1, 1, 10, 20, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}
}

func TestGetNextExecutionTime(t *testing.T) {
	expr := "0 0 12 * * 1-5 *"
	now := time.Date(2023, 1, 7, 10, 15, 30, 0, time.UTC) // 周六

	next, err := GetNextExecutionTime(expr, now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}

	expected := time.Date(2023, 1, 9, 12, 0, 0, 0, time.UTC) // 下周一
	if !next.Equal(expected) {
		t.Errorf("下一次执行时间错误: 期望 %v, 得到 %v", expected, next)
	}

	// 测试无效表达式
	expr = "0 0 12 * * 1-5"
	_, err = GetNextExecutionTime(expr, now)
	if err == nil {
		t.Errorf("使用无效表达式获取下一次执行时间应该失败，但成功了")
	}

	// 测试带年份的表达式
	expr = "0 0 12 * * 1-5 2024"
	next, err = GetNextExecutionTime(expr, now)
	if err != nil {
		t.Fatalf("获取带年份的下一次执行时间失败: %v", err)
	}

	// 应该在2024年
	if next.Year() != 2024 {
		t.Errorf("下一次执行时间应该在2024年，但得到 %v", next.Year())
	}
}

func TestCronJobExecuteTime(t *testing.T) {
	expr := "* * * 6 8 * */5"
	cron, err := NewCronExpression(expr)
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}
	now := time.Now()
	for i := 0; i < 20; i++ {
		next, err := cron.NextExecutionTime(now)
		if err != nil {
			t.Fatalf("获取下一次执行时间失败: %v", err)
		}
		if next.Before(now) {
			t.Errorf("下一次执行时间应该在当前时间之后")
		}
		t.Log("下次执行的时间是", next)
		// 添加时间
		now = now.Add(time.Hour * 24 * 365)
	}

	t.Log("最后一次判断执行时间")
	next, err := cron.NextExecutionTime(now)
	if err != nil {
		t.Fatalf("获取下一次执行时间失败: %v", err)
	}
	t.Log("下次执行的时间是", next)
}
