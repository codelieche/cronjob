package config

import (
	"fmt"
	"strconv"
)

// database 数据库配置
type database struct {
	Driver   string // 数据库的driver：mysql, postgresql, postgres
	Host     string // 数据库地址
	Port     int    // 数据库端口
	Database string // 数据库
	User     string // 数据库用户
	Password string // 数据库密码
	Schema   string // PG数据库的schema
}

// GetDSN 获取数据库的DSN
func (db *database) GetDSN() string {
	// 🔥 使用Asia/Shanghai时区，确保与MySQL时区一致（URL编码为Asia%2FShanghai）
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&collation=utf8mb4_0900_ai_ci",
		db.User, db.Password, db.Host, db.Port, db.Database)
	// PG数据库的话默认的schema是public
	if db.Driver == "postgresql" || db.Driver == "postgres" {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d search_path=%s sslmode=disable TimeZone=Asia/Shanghai",
			db.Host, db.User, db.Password, db.Database, db.Port, db.Schema)
	}
	return dsn
}

// Database 数据库配置
var Database *database

// parseDatabase 解析数据库配置
func parseDatabase() {
	driver := GetDefaultEnv("DB_DRIVER", "mysql")
	host := GetDefaultEnv("DB_HOST", "127.0.0.1")
	portStr := GetDefaultEnv("DB_PORT", "3306")
	dbName := GetDefaultEnv("DB_NAME", "cronjob_apiserver")
	user := GetDefaultEnv("DB_USER", "root")
	password := GetDefaultEnv("DB_PASSWORD", "root")
	schema := GetDefaultEnv("DB_SCHEMA", "public")

	// 解析端口
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 3306
	}

	// 修改database的值
	Database = &database{
		driver,
		host,
		port,
		dbName,
		user,
		password,
		schema,
	}
}

func init() {
	parseDatabase()
}
