package types

type PaginationConfig struct {
	MaxPage            int    // 列表中我们可以获取的最大页数, 默认: 1000
	PageQueryParam     string // 默认是: page
	MaxPageSize        int    // 最大的页数：默认：100
	PageSizeQueryParam string // 默认是: page_size
}

type Pagination struct {
	Page     int `json:"page" form:"page"`           // 页码
	PageSize int `json:"page_size" form:"page_size"` // 每页数据大小
}
