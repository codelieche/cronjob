package types

/**
Http相关的结构体
*/

// Response http返回的响应
type Response struct {
	Code    int         `json:"code"`              // 返回的code，如果是0就表示正常
	Data    interface{} `json:"data,omitempty"`    // 返回的数据
	Message string      `json:"message,omitempty"` // 返回的消息
}

// ResponseList http返回列表数据
type ResponseList struct {
	Page     int         `json:"page"`      // 当前页码
	PageSize int         `json:"page_size"` // 当前页码大小
	Count    int64       `json:"count"`     // 当前列表的数据
	Results  interface{} `json:"results"`   // 返回的列表数据
}
