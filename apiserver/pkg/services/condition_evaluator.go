// Package services 业务逻辑服务层
//
// 条件评估引擎 - 用于工作流条件分支的条件判断
package services

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// ConditionEvaluator 条件评估器
//
// 负责评估工作流步骤的条件表达式，支持：
// 1. 简单状态条件：success, failed, error, timeout, stopped, canceled
// 2. 表达式条件：使用 antonmedv/expr 库，支持复杂的布尔表达式
//
// 表达式中可以访问的变量：
// - Variables: 工作流变量（如 deploy_env, branch, version）
// - task_status: 上一步的详细状态（success/failed/error/timeout/stopped/canceled）
// - output: 上一步的输出对象（如 output.code, output.status）
// - output 的顶层字段也可以直接访问（如 code, status）
//
// 性能优化：
// - 编译后的表达式程序会被缓存，避免重复编译
// - 简单状态条件使用快速路径，无需表达式引擎
type ConditionEvaluator struct {
	// cache 编译后的表达式程序缓存（表达式 → 编译后的程序）
	// 使用缓存可以大幅提升性能，避免每次都重新编译相同的表达式
	cache map[string]*vm.Program

	// mutex 读写锁，保护 cache 的并发访问
	mutex sync.RWMutex
}

// NewConditionEvaluator 创建条件评估器实例
//
// 返回：
//   - *ConditionEvaluator: 新的条件评估器实例
func NewConditionEvaluator() *ConditionEvaluator {
	return &ConditionEvaluator{
		cache: make(map[string]*vm.Program),
	}
}

// Evaluate 评估条件表达式
//
// 这是核心评估方法，支持两种模式：
// 1. 简单状态条件：直接比较字符串（快速路径）
// 2. 表达式条件：使用 expr 库评估复杂表达式
//
// 参数：
//   - condition: 条件表达式字符串
//   - context: 评估上下文（变量字典）
//
// 返回：
//   - bool: 条件是否满足
//   - error: 评估错误（表达式语法错误、编译失败、执行失败等）
//
// 示例：
//
//	// 简单状态条件
//	evaluator.Evaluate("success", map[string]interface{}{
//	    "task_status": "success",
//	})
//
//	// 表达式条件
//	evaluator.Evaluate("exit_code == 0 && deploy_env == 'production'", map[string]interface{}{
//	    "exit_code": 0,
//	    "deploy_env": "production",
//	})
func (e *ConditionEvaluator) Evaluate(condition string, context map[string]interface{}) (bool, error) {
	// ========== Step 1: 处理空条件 ==========
	// 空条件意味着无条件执行，总是返回 true
	if condition == "" {
		return true, nil
	}

	// ========== Step 2: 🔥 处理简单状态条件（快速路径）==========
	// 支持所有任务状态常量作为简单条件
	// 例如："success" 等同于 "task_status == 'success'"
	simpleStatuses := []string{
		"success",  // 执行成功
		"failed",   // 业务失败
		"error",    // 系统错误
		"timeout",  // 执行超时
		"stopped",  // 被停止
		"canceled", // 被取消
		"skipped",  // 被跳过（用于查询，通常不用于条件）
	}

	// 检查是否是简单状态条件
	for _, status := range simpleStatuses {
		if condition == status {
			// 从 context 中获取 task_status
			if lastStatus, ok := context["task_status"].(string); ok {
				// 🔥 特殊处理：condition="failed" 匹配所有失败类型
				// 原因：从用户角度看，error/timeout 也是失败，应该触发失败分支
				if condition == "failed" {
					failureStatuses := []string{"failed", "error", "timeout"}
					for _, fs := range failureStatuses {
						if lastStatus == fs {
							return true, nil
						}
					}
					return false, nil
				}

				// 其他状态：严格匹配
				return lastStatus == condition, nil
			}
			// 如果 context 中没有 task_status，说明调用方式有误
			return false, fmt.Errorf("context 中缺少 task_status 字段")
		}
	}

	// ========== Step 3: 🔥 替换变量语法 ==========
	// 将 ${variable} 语法替换为 expr 库支持的 variable 语法
	// 例如：${exit_code} → exit_code
	// 例如：${step_1.output.code} → step_1.output.code
	expression := replaceVariableSyntax(condition)

	// ========== Step 4: 获取或编译表达式程序 ==========
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("编译表达式失败: %w", err)
	}

	// ========== Step 5: 执行表达式程序 ==========
	output, err := expr.Run(program, context)
	if err != nil {
		return false, fmt.Errorf("执行表达式失败: %w", err)
	}

	// ========== Step 6: 转换结果为 bool ==========
	// expr 库保证了返回类型是 bool（因为我们在编译时指定了 expr.AsBool()）
	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("表达式结果不是 bool 类型: %T", output)
	}

	return result, nil
}

// EvaluateWithLastStatus 评估条件（带上一步状态和输出）⭐⭐⭐
//
// 这是工作流执行时使用的主要方法，提供完整的上下文信息：
// 1. Variables: 工作流变量
// 2. lastStatus: 上一步的详细状态
// 3. lastOutput: 上一步的输出对象
//
// 构建的评估上下文包含：
// - 所有 Variables 的键值对
// - task_status: 上一步的状态
// - output: 上一步的输出对象（可以访问 output.code, output.status 等）
// - output 的所有顶层键值对（可以直接访问 code, status 等）
//
// 参数：
//   - condition: 条件表达式
//   - variables: 工作流变量（从 WorkflowExecute.GetVariables() 获取）
//   - lastStatus: 上一步的详细状态（success/failed/error/timeout/stopped/canceled）
//   - lastOutput: 上一步的输出对象（从 Task.Output 解析）
//
// 返回：
//   - bool: 条件是否满足
//   - error: 评估错误
//
// 示例：
//
//	// 场景：上一步健康检查返回 503（业务失败）
//	evaluator.EvaluateWithLastStatus(
//	    "task_status == 'failed'",  // 条件：上一步业务失败
//	    map[string]interface{}{"deploy_env": "production"},  // 工作流变量
//	    "failed",  // 上一步状态
//	    map[string]interface{}{"code": 503, "message": "Service Unavailable"},  // 上一步输出
//	)
//	// 返回: true, nil
//
//	// 场景：根据输出 code 判断
//	evaluator.EvaluateWithLastStatus(
//	    "output.code == 0 && deploy_env == 'production'",
//	    map[string]interface{}{"deploy_env": "production"},
//	    "success",
//	    map[string]interface{}{"code": 0, "status": "healthy"},
//	)
//	// 返回: true, nil
func (e *ConditionEvaluator) EvaluateWithLastStatus(
	condition string,
	variables map[string]interface{},
	lastStatus string,
	lastOutput map[string]interface{},
) (bool, error) {
	// ========== Step 1: 构建完整的评估上下文 ==========
	context := make(map[string]interface{})

	// 1.1 复制工作流变量
	for k, v := range variables {
		context[k] = v
	}

	// 1.2 🔥 添加上一步的详细状态（关键变量）
	// 可以用于条件表达式：task_status == 'failed'
	context["task_status"] = lastStatus

	// 1.3 🔥 添加上一步的输出对象
	if len(lastOutput) > 0 {
		// 方式1：作为 output 对象（支持 output.code, output.status 语法）
		context["output"] = lastOutput

		// 方式2：将输出的顶层字段也添加到 context（支持直接访问 code, status）
		// 注意：只添加不冲突的字段，避免覆盖工作流变量
		for k, v := range lastOutput {
			if _, exists := context[k]; !exists {
				context[k] = v
			}
		}
	}

	// ========== Step 2: 调用核心评估方法 ==========
	return e.Evaluate(condition, context)
}

// getOrCompileProgram 获取或编译表达式程序（带缓存）
//
// 这是性能优化的关键方法：
// 1. 首先尝试从缓存读取已编译的程序
// 2. 如果缓存未命中，则编译表达式并存入缓存
// 3. 使用读写锁保证并发安全
//
// 编译选项：
// - expr.AsBool(): 强制返回值为 bool 类型
// - expr.AllowUndefinedVariables(): 允许访问未定义的变量（返回 nil）
//
// 参数：
//   - expression: 表达式字符串（已经过变量语法替换）
//
// 返回：
//   - *vm.Program: 编译后的程序
//   - error: 编译错误
func (e *ConditionEvaluator) getOrCompileProgram(expression string) (*vm.Program, error) {
	// ========== Step 1: 尝试从缓存读取（使用读锁）==========
	e.mutex.RLock()
	if program, ok := e.cache[expression]; ok {
		e.mutex.RUnlock()
		return program, nil
	}
	e.mutex.RUnlock()

	// ========== Step 2: 缓存未命中，编译表达式 ==========
	program, err := expr.Compile(expression,
		expr.AsBool(),                  // 🔥 强制返回值为 bool 类型
		expr.AllowUndefinedVariables(), // 🔥 允许未定义的变量（返回 nil，避免报错）
	)
	if err != nil {
		return nil, err
	}

	// ========== Step 3: 存入缓存（使用写锁）==========
	e.mutex.Lock()
	e.cache[expression] = program
	e.mutex.Unlock()

	return program, nil
}

// replaceVariableSyntax 替换变量语法
//
// 将 ${variable} 语法替换为 expr 库支持的 variable 语法
//
// 转换规则：
// - ${variable} → variable
// - ${step_1.output.code} → step_1.output.code
// - ${data[0].value} → data[0].value
//
// 参数：
//   - condition: 原始条件字符串
//
// 返回：
//   - string: 替换后的条件字符串
//
// 示例：
//
//	replaceVariableSyntax("${exit_code} == 0")
//	// 返回: "exit_code == 0"
//
//	replaceVariableSyntax("${output.code} == 0 && ${deploy_env} == 'production'")
//	// 返回: "output.code == 0 && deploy_env == 'production'"
func replaceVariableSyntax(condition string) string {
	// 使用正则表达式匹配 ${...} 并替换为 ...
	// 正则说明：
	// - \$\{: 匹配 ${
	// - ([^}]+): 捕获组，匹配除 } 外的所有字符
	// - \}: 匹配 }
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllString(condition, "$1")
}
