package datasource

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
)

func TestMigrateDB(t *testing.T) {
	log.Println(
		"是否有departments表：", DB.HasTable("departments"),
	)
	log.Println(
		"是否有users表：", DB.HasTable(&datamodels.User{}),
	)
	defer DB.Close()
}
