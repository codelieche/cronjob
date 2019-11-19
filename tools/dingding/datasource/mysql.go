package datasource

import (
	"fmt"
	"log"
	"os"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datamodels"

	"github.com/jinzhu/gorm"
	//_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/go-sql-driver/mysql"
)

var DB *gorm.DB
var config *common.Config

func init() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// 实例化DingDing

	if config == nil {
		if err := common.ParseConfig(); err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
		config = common.GetConfig()
	}

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
	// log.Println(msqlUri)

	DB, err = gorm.Open("mysql", msqlUri)
	//db, err = gorm.Open("sqlite3", "dingding.db")
	if err != nil {
		log.Println(err.Error())
		//panic(err)
		os.Exit(1)
	} else {
		//defer db.Close()
	}

	// Migrate the schema
	DB.AutoMigrate(&datamodels.Department{})
	DB.AutoMigrate(&datamodels.User{})
	DB.AutoMigrate(&datamodels.Message{})

	// 显示SQL
	DB.LogMode(config.Debug)
}

func Close() {
	defer DB.Close()
}
