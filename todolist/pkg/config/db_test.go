package config

import (
	"testing"
)

func TestDatabase_GetDSN(t *testing.T) {
	Database = &database{
		Driver:   "mysql",
		Host:     "127.0.0.1",
		Port:     3306,
		Database: "test",
		User:     "root",
		Password: "root",
		Schema:   "public",
	}
	dsn := Database.GetDSN()
	if dsn != "root:root@tcp(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local" {
		t.Errorf("GetDSN failed, expect: root:root@tcp(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local, actual: %s", dsn)
	}
}
