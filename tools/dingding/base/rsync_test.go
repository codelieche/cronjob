package base

import (
	"log"
	"testing"
)

// 同步钉钉数据
func TestRsyncDingDingData(t *testing.T) {
	defer db.Close()
	if err := RsyncDingDingData(); err != nil {
		t.Error(err)
	} else {
		log.Println("rsync Done!")
	}
}
