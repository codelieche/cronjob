package datasources

import (
	"log"
	"testing"
)

func TestGetMongoDB(t *testing.T) {
	if m := GetMongoDB(); m != nil {
		log.Println(m)
	} else {
		t.Error("获取Mongo出错")
	}
}
