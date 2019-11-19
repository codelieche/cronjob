package repositories

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datasource"
)

// 同步钉钉数据
func TestRsyncDingDingData(t *testing.T) {
	db := datasource.DB
	ding := common.NewDing()
	defer db.Close()
	if err := RsyncDingDingData(ding); err != nil {
		t.Error(err)
	} else {
		log.Println("rsync Done!")
	}
}
