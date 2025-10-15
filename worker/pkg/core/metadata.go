// Package core 核心数据模型和接口定义
//
// 包含系统中所有核心业务实体的数据模型定义
// 以及相关的数据访问接口和服务接口
package core

import (
	"encoding/json"
)

// Metadata 统一元数据结构（精简版）
//
// 用于定义任务执行环境和配置信息，支持：
// - CronJob: 定义默认执行环境
// - Workflow: 锁定执行环境（同 Worker、同目录）
// - Task: 继承并合并配置
//
// 设计原则：
// - 简单够用：只保留 6 个核心字段
// - 统一结构：CronJob、Workflow、Task 都使用这个结构
// - 向后兼容：JSON 自动忽略无效字段
type Metadata struct {
	// ========== 执行环境配置（3 个核心字段）==========

	// WorkingDir 工作目录
	// - 留空: 使用默认目录 ./tasks/{task_id}/ 或 ./tasks/workflow-{id}/
	// - 指定: 使用指定目录（绝对路径或相对路径）
	// - Workflow 锁定后: 强制使用统一目录
	// 🔥 统一使用下划线命名（与前端保持一致，与旧版本驼峰命名不兼容）
	WorkingDir string `json:"working_dir,omitempty"`

	// WorkerSelect 可选 Worker 列表
	// - 空数组: 所有 Worker 都可以执行
	// - 非空: 只能在指定 Worker 上执行（如：["worker-prod-01"]）
	// - Workflow 锁定后: 缩小为单个 Worker
	WorkerSelect []string `json:"worker_select,omitempty"`

	// Environment 环境变量
	// 在任务执行时注入到进程环境
	// 如：{"NODE_ENV": "production", "LOG_LEVEL": "info"}
	Environment map[string]string `json:"environment,omitempty"`

	// ========== Workflow 标识（2 个字段）==========

	// WorkflowID Workflow ID
	// 标记此 Task 属于某个 Workflow
	// 普通 CronJob Task 此字段为空
	WorkflowID string `json:"workflow_id,omitempty"`

	// StepOrder 步骤序号
	// 在 Workflow 中的执行顺序（从 1 开始）
	// 普通 CronJob Task 此字段为 0
	StepOrder int `json:"step_order,omitempty"`

	// ========== 其他配置（1 个字段）==========

	// Priority 优先级（1-10，默认 5）
	// 数值越大优先级越高
	// 用于任务调度时的优先级排序
	Priority int `json:"priority,omitempty"`
}

// MergeMetadata 合并元数据（精简版）
//
// 将父级 Metadata 和子级 Metadata 合并，支持：
// - 普通字段：子级覆盖父级
// - Map 字段（environment）：合并，子级覆盖同名 key
// - Workflow 锁定字段：子级不可覆盖（locked=true）
//
// 参数：
//   - parent: 父级 Metadata（CronJob 或 Workflow）
//   - child: 子级 Metadata（Task 或自定义配置）
//   - locked: 是否锁定（Workflow 第二个及之后的 Step 为 true）
//
// 返回：
//   - 合并后的 Metadata
//
// 示例：
//
//	parent := &Metadata{
//	    WorkingDir: "/data/projects/myapp",
//	    Environment: map[string]string{"APP_ENV": "production"},
//	    Priority: 5,
//	}
//	child := &Metadata{
//	    Environment: map[string]string{"LOG_LEVEL": "debug"},
//	}
//	result := MergeMetadata(parent, child, false)
//	// result.WorkingDir = "/data/projects/myapp"
//	// result.Environment = {"APP_ENV": "production", "LOG_LEVEL": "debug"}
//	// result.Priority = 5
func MergeMetadata(parent, child *Metadata, locked bool) *Metadata {
	result := &Metadata{}

	// 1. 从父级复制所有字段
	if parent != nil {
		result.WorkingDir = parent.WorkingDir
		result.Priority = parent.Priority
		result.WorkflowID = parent.WorkflowID
		result.StepOrder = parent.StepOrder

		// 深拷贝 WorkerSelect
		if len(parent.WorkerSelect) > 0 {
			result.WorkerSelect = make([]string, len(parent.WorkerSelect))
			copy(result.WorkerSelect, parent.WorkerSelect)
		}

		// 深拷贝 Environment
		if len(parent.Environment) > 0 {
			result.Environment = make(map[string]string)
			for k, v := range parent.Environment {
				result.Environment[k] = v
			}
		}
	}

	// 2. 子级覆盖（如果允许）
	if child != nil {
		// ⭐ 关键：locked=true 时，working_dir 和 worker_select 不可覆盖
		if !locked {
			if child.WorkingDir != "" {
				result.WorkingDir = child.WorkingDir
			}
			if len(child.WorkerSelect) > 0 {
				result.WorkerSelect = make([]string, len(child.WorkerSelect))
				copy(result.WorkerSelect, child.WorkerSelect)
			}
		}

		// Environment 总是可以扩展（子级覆盖同名 key）
		if len(child.Environment) > 0 {
			if result.Environment == nil {
				result.Environment = make(map[string]string)
			}
			for k, v := range child.Environment {
				result.Environment[k] = v
			}
		}

		// Priority 可以覆盖
		if child.Priority > 0 {
			result.Priority = child.Priority
		}

		// WorkflowID 和 StepOrder 可以覆盖（用于 Workflow Task）
		if child.WorkflowID != "" {
			result.WorkflowID = child.WorkflowID
		}
		if child.StepOrder > 0 {
			result.StepOrder = child.StepOrder
		}
	}

	return result
}

// ParseMetadata 解析 JSON 格式的元数据
//
// 将 json.RawMessage 解析为 Metadata 结构体
// 如果 JSON 为空，返回空的 Metadata（不是 nil）
//
// 参数：
//   - data: JSON 格式的元数据
//
// 返回：
//   - 解析后的 Metadata 结构体
//   - 解析错误（如果有）
//
// 示例：
//
//	rawJSON := json.RawMessage(`{"working_dir": "/data", "priority": 8}`)
//	metadata, err := ParseMetadata(rawJSON)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(metadata.WorkingDir)  // 输出: /data
//	fmt.Println(metadata.Priority)    // 输出: 8
func ParseMetadata(data json.RawMessage) (*Metadata, error) {
	if len(data) == 0 {
		return &Metadata{}, nil
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// SerializeMetadata 将 Metadata 序列化为 JSON
//
// 将 Metadata 结构体序列化为 json.RawMessage
// 用于保存到数据库
//
// 参数：
//   - metadata: Metadata 结构体
//
// 返回：
//   - JSON 格式的元数据
//   - 序列化错误（如果有）
//
// 示例：
//
//	metadata := &Metadata{
//	    WorkingDir: "/data",
//	    Priority: 8,
//	}
//	rawJSON, err := SerializeMetadata(metadata)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(string(rawJSON))  // 输出: {"working_dir":"/data","priority":8}
func SerializeMetadata(metadata *Metadata) (json.RawMessage, error) {
	if metadata == nil {
		return json.RawMessage("{}"), nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	return data, nil
}
