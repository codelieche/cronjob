package handlers

import "github.com/codelieche/cronjob/backend/common"

// Job List Response
type JobListResponse struct {
	Count   int           `json:"count"`   // Jobs的个数
	Next    string        `json:"next"`    // 下一页
	Results []*common.Job `json:"results"` // Job的列表
}
