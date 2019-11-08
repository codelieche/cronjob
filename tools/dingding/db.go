package dingding

import (
	"log"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var db *gorm.DB
var ding *DingDing

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// 实例化DingDing
	ding = NewDing()

	// 连接数据库
	log.Println(os.Getenv("PWD"))
	var err error
	db, err = gorm.Open("sqlite3", "dingding2.db")
	if err != nil {
		panic(err)
		return
	} else {
		//defer db.Close()
	}

	// Migrate the schema
	db.AutoMigrate(&Department{})
	db.AutoMigrate(&User{})
}
