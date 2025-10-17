package forms

import (
	"time"
)

// TodoListCreateForm 创建待办事项表单
type TodoListCreateForm struct {
	ParentID    *string                `json:"parent_id" example:"123e4567-e89b-12d3-a456-426614174000"`          // 父待办事项ID，可选，支持层级化管理
	Project     string                 `json:"project" binding:"max=128" example:"webapp"`                        // 项目代码，可选，最大128字符
	Title       string                 `json:"title" binding:"required,max=512" example:"完成项目文档"`                 // 待办事项标题，必填，最大512字符
	Description string                 `json:"description" binding:"max=2000" example:"完成项目的详细文档，包括API接口说明和使用示例"` // 待办事项描述，可选，最大2000字符
	Priority    int                    `json:"priority" binding:"min=1,max=5" example:"3"`                        // 优先级，1-5，默认1
	Category    string                 `json:"category" binding:"max=128" example:"工作"`                           // 分类，可选，最大128字符
	Tags        string                 `json:"tags" binding:"max=512" example:"文档,项目,重要"`                         // 标签，以逗号分隔，可选，最大512字符
	StartTime   *time.Time             `json:"start_time" example:"2024-12-01T09:00:00Z"`                         // 开始时间，可选，用于时间段任务
	Deadline    *time.Time             `json:"deadline" example:"2024-12-31T23:59:59Z"`                           // 截止期限，可选
	Progress    *int                   `json:"progress" binding:"omitempty,min=0,max=100" example:"50"`           // 手动完成进度（0-100），可选
	Metadata    map[string]interface{} `json:"metadata" swaggertype:"object"`                                     // 元数据，可选的自定义信息
}

// TodoListUpdateForm 更新待办事项表单
type TodoListUpdateForm struct {
	ParentID    *string                `json:"parent_id" example:"123e4567-e89b-12d3-a456-426614174000"`               // 父待办事项ID，可选，支持层级化管理
	Project     string                 `json:"project" binding:"max=128" example:"webapp"`                             // 项目代码，可选，最大128字符
	Title       string                 `json:"title" binding:"required,max=512" example:"完成项目文档"`                      // 待办事项标题，必填，最大512字符
	Description string                 `json:"description" binding:"max=2000" example:"完成项目的详细文档，包括API接口说明和使用示例"`      // 待办事项描述，可选，最大2000字符
	Status      string                 `json:"status" binding:"oneof=pending running done canceled" example:"running"` // 状态，必须是有效状态之一
	Priority    int                    `json:"priority" binding:"min=1,max=5" example:"3"`                             // 优先级，1-5
	Category    string                 `json:"category" binding:"max=128" example:"工作"`                                // 分类，可选，最大128字符
	Tags        string                 `json:"tags" binding:"max=512" example:"文档,项目,重要"`                              // 标签，以逗号分隔，可选，最大512字符
	StartTime   *time.Time             `json:"start_time" example:"2024-12-01T09:00:00Z"`                              // 开始时间，可选，用于时间段任务
	Deadline    *time.Time             `json:"deadline" example:"2024-12-31T23:59:59Z"`                                // 截止期限，可选
	Progress    *int                   `json:"progress" binding:"omitempty,min=0,max=100" example:"50"`                // 手动完成进度（0-100），可选
	Metadata    map[string]interface{} `json:"metadata" swaggertype:"object"`                                          // 元数据，可选的自定义信息
}

// TodoListPatchForm 部分更新待办事项表单
type TodoListPatchForm struct {
	ParentID    *string                `json:"parent_id,omitempty" example:"123e4567-e89b-12d3-a456-426614174000"`                         // 父待办事项ID，可选，支持层级化管理
	Project     *string                `json:"project,omitempty" binding:"omitempty,max=128" example:"webapp"`                             // 项目代码，可选，最大128字符
	Title       *string                `json:"title,omitempty" binding:"omitempty,max=512" example:"完成项目文档"`                               // 待办事项标题，可选，最大512字符
	Description *string                `json:"description,omitempty" binding:"omitempty,max=2000" example:"完成项目的详细文档"`                     // 待办事项描述，可选，最大2000字符
	Status      *string                `json:"status,omitempty" binding:"omitempty,oneof=pending running done canceled" example:"running"` // 状态，可选，必须是有效状态之一
	Priority    *int                   `json:"priority,omitempty" binding:"omitempty,min=1,max=5" example:"3"`                             // 优先级，可选，1-5
	Category    *string                `json:"category,omitempty" binding:"omitempty,max=128" example:"工作"`                                // 分类，可选，最大128字符
	Tags        *string                `json:"tags,omitempty" binding:"omitempty,max=512" example:"文档,项目,重要"`                              // 标签，可选，最大512字符
	StartTime   *time.Time             `json:"start_time,omitempty" example:"2024-12-01T09:00:00Z"`                                        // 开始时间，可选，用于时间段任务
	Deadline    *time.Time             `json:"deadline,omitempty" example:"2024-12-31T23:59:59Z"`                                          // 截止期限，可选
	Progress    *int                   `json:"progress,omitempty" binding:"omitempty,min=0,max=100" example:"50"`                          // 手动完成进度（0-100），可选
	Metadata    map[string]interface{} `json:"metadata,omitempty" swaggertype:"object"`                                                    // 元数据，可选的自定义信息
}

// TodoListStatusUpdateForm 状态更新表单
type TodoListStatusUpdateForm struct {
	Status string `json:"status" binding:"required,oneof=pending running done canceled" example:"done"` // 状态，必填，必须是有效状态之一
}

// TodoListQueryForm 查询表单
type TodoListQueryForm struct {
	Status   string `form:"status" binding:"omitempty,oneof=pending running done canceled" example:"pending"` // 状态过滤，可选
	Project  string `form:"project" binding:"omitempty,max=128" example:"webapp"`                             // 项目代码过滤，可选
	Category string `form:"category" binding:"omitempty,max=128" example:"工作"`                                // 分类过滤，可选
	Priority *int   `form:"priority" binding:"omitempty,min=1,max=5" example:"3"`                             // 优先级过滤，可选
	Tags     string `form:"tags" binding:"omitempty,max=512" example:"重要"`                                    // 标签过滤，可选
	Search   string `form:"search" binding:"omitempty,max=256" example:"项目"`                                  // 搜索关键词，可选
	Page     int    `form:"page" binding:"omitempty,min=1" example:"1"`                                       // 页码，可选，默认1
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100" example:"10"`                         // 每页大小，可选，默认10，最大100
	Ordering string `form:"ordering" binding:"omitempty" example:"-created_at"`                               // 排序规则，可选，支持字段名和-字段名
}
