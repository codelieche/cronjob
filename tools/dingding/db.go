package dingding

import (
	"fmt"
	"log"
	"os"

	"github.com/jinzhu/gorm"
	//_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/go-sql-driver/mysql"
)

var db *gorm.DB
var ding *DingDing

func init() {
	var (
		mysqlHost     string
		mysqlPort     string
		mysqlUser     string
		mysqlPassword string
		mysqlDbName   string
	)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// 实例化DingDing
	ding = NewDing()

	// 连接数据库
	log.Println(os.Getenv("PWD"))
	var err error
	mysqlHost = os.Getenv("MYSQL_HOST")
	mysqlPort = os.Getenv("MYSQL_PORT")
	mysqlUser = os.Getenv("MYSQL_USER")
	mysqlPassword = os.Getenv("MYSQL_PASSWORD")
	mysqlDbName = os.Getenv("MYSQL_DB_NAME")

	//msqlUri := fmt.Sprintf("%s:%s@(%s:%s)/%s",
	msqlUri := fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDbName)
	log.Println(mysqlHost, mysqlPort, mysqlUser, mysqlDbName)

	db, err = gorm.Open("mysql", msqlUri)
	//db, err = gorm.Open("sqlite3", "dingding.db")
	if err != nil {
		log.Println(err.Error())
		//panic(err)
		os.Exit(1)
	} else {
		//defer db.Close()
	}

	// Migrate the schema
	db.AutoMigrate(&Department{})
	db.AutoMigrate(&User{})
}

func Close() {
	defer db.Close()
}
