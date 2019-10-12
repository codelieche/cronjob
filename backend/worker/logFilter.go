package worker

type JobExecuteLogFilter struct {
	Name string `bson: "name"` // job的名字

}

type SortLogByStartTime struct {
	StartTime int `bson: "startTime"` // 根据开始时间排序
}
