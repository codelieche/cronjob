package worker

import (
	"testing"

	"github.com/codelieche/cronjob/backend/common"
)

// 测试获取日志列表
// 如果执行提示出错，可注释掉init中的parseParamse()
func TestMongoLogHandler_List(t *testing.T) {
	if logHandler, err := NewMongoLogHandler(common.Config.Worker.Mongo); err != nil {
		t.Error(err)
	} else {
		if logList, err := logHandler.List(1, 10); err != nil {
			t.Error(err)
		} else {
			t.Log("获取到日志的长度：", len(logList))
			for _, l := range logList {
				t.Logf("%-10s:\t%s", l.Name, l.StartTime)
			}
		}
	}
}
