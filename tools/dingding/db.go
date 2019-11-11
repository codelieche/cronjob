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

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// 实例化DingDing

	if config == nil {
		if err := ParseConfig(); err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	}

	ding = NewDing()

	// 连接数据库
	// log.Println(os.Getenv("PWD"))
	var err error
	//mysqlHost = os.Getenv("MYSQL_HOST")
	//mysqlPort = os.Getenv("MYSQL_PORT")
	//mysqlUser = os.Getenv("MYSQL_USER")
	//mysqlPassword = os.Getenv("MYSQL_PASSWORD")
	//mysqlDbName = os.Getenv("MYSQL_DB_NAME")

	//log.Println(mysqlHost, mysqlPort, mysqlUser, mysqlDbName)

	//msqlUri := fmt.Sprintf("%s:%s@(%s:%s)/%s",
	msqlUri := fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Database.User, config.Database.Password,
		config.Database.Host, config.Database.Port, config.Database.Database)

	// log.Println(config.Database.User, config.Database.Host, config.Database.Port, config.Database.Database)

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
