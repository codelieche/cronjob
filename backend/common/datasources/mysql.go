package datasources

import (
	"fmt"
	"log"
	"os"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"github.com/codelieche/cronjob/backend/common"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

var db *gorm.DB

var config *common.MasterWorkerConfig

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
			config = common.Config
		}
	}

	// 2. 连接数据库
	// 2-1: 获取mysqlUri
	log.Println(*config.Master.MySQL)
	mysqlUri = fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Master.MySQL.User, config.Master.MySQL.Password,
		config.Master.MySQL.Host, config.Master.MySQL.Port, config.Master.MySQL.Database)
	log.Println(mysqlUri)
	// 2-2: 连接数据库
	db, err = gorm.Open("mysql", mysqlUri)

	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	} else {

	}

	// 3. Migrate the Schema
	db.AutoMigrate(&datamodels.Category{})
	db.AutoMigrate(&datamodels.Job{})

	//
	db.LogMode(config.Debug)

}

func GetDb() *gorm.DB {
	if db != nil {
		return db
	} else {
		initDb()
		return db
	}
}
