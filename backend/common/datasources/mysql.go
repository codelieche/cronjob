package datasources

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

var db *gorm.DB

var config *common.Config

func initDb() {
	log.SetFlags(log.Lshortfile)
	var (
		err      error
		mysqlUri string
	)

	// 1. 先获取配置
	if config == nil {
		if err = common.ParseConfig(); err != nil {
			log.Println(err.Error())
			os.Exit(1)
		} else {
			config = common.GetConfig()
		}
	}

	// 2. 连接数据库
	// 2-1: 获取mysqlUri
	log.Println(*config.MySQL)
	mysqlUri = fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.MySQL.User, config.MySQL.Password,
		config.MySQL.Host, config.MySQL.Port, config.MySQL.Database)
	log.Println(mysqlUri)
	// 2-2: 连接数据库
	db, err = gorm.Open("mysql", mysqlUri)
	//db2, err := sql.Open("mysql", mysqlUri)
	//db2.Ping()

	//sql.Open()

	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {

	}

	// 3. Migrate the Schema
	db.AutoMigrate(&datamodels.Category{})
	db.AutoMigrate(&datamodels.Job{})
	db.AutoMigrate(&datamodels.JobKill{})
	db.AutoMigrate(&datamodels.JobExecute{})

	//
	db.LogMode(config.Debug)

	//  packets.go:36: unexpected EOF
	// db.DB()是 *sql.DB
	// SHOW GLOBAL VARIABLES LIKE '%timeout%';
	// SET GLOBAL wait_timeout=300;
	db.DB().SetConnMaxLifetime(120 * time.Second) // 给db设置一个超时时间，小于数据库的超时时间
	db.DB().SetMaxOpenConns(100)                  // 设置最大打开的连接数，默认是0，表示不限制
	db.DB().SetMaxIdleConns(20)                   // 设置最大空闲连接数
	//log.Println(db.DB().Ping())

}

func GetDb() *gorm.DB {
	if db != nil {
		return db
	} else {
		initDb()
		return db
	}
}
