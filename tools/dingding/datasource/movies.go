package datasource

import "github.com/codelieche/cronjob/tools/dingding/datamodels"

var Movies = map[int64]*datamodels.Movie{
	1: {
		Title:       "电影01",
		Description: "描述01",
	},
	2: {
		Title:       "电影02",
		Description: "描述02",
	},
	3: {
		Title:       "电影03",
		Description: "描述03",
	},
}

var MovieID int64 = 3
