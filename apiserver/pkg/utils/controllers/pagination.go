package controllers

import (
	"github.com/codelieche/cronjob/apiserver/pkg/utils/types"
)

var pageConfig *types.PaginationConfig

func SetPaginationConfig(config *types.PaginationConfig) {
	pageConfig = config
}

func init() {
	defaultPaginationConfig := &types.PaginationConfig{
		MaxPage:            1000,
		PageQueryParam:     "page",
		MaxPageSize:        300,
		PageSizeQueryParam: "page_size",
	}

	SetPaginationConfig(defaultPaginationConfig)
}
