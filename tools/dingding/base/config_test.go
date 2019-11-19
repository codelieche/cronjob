package dingding

import (
	"log"
	"os"
	"testing"
)

func TestParseConfig(t *testing.T) {
	log.Println(os.Getenv("PWD"))
	if err := ParseConfig(); err != nil {
		t.Error(err.Error())

	} else {
		log.Println("解析配置文件成功")
	}
}
