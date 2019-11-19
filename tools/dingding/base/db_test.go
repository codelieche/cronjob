package base

import (
	"log"
	"testing"
)

func TestMigrateDB(t *testing.T) {
	log.Println(
		"是否有departments表：", db.HasTable("departments"),
	)
	log.Println(
		"是否有users表：", db.HasTable(&User{}),
	)
	defer db.Close()
}
