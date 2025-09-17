package core

import (
	"github.com/codelieche/cronjob/apiserver/pkg/config"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// db 全局数据库连接实例
var db *gorm.DB

// init 初始化函数，在包加载时自动连接数据库
func init() {
	// 初始化数据库连接
	var err error
	if db, err = connectDatabase(); err != nil {
		logger.Panic("数据库初始化连接失败", zap.Error(err))
	}
}

// GetDB 获取数据库连接实例
// 如果连接不存在，会尝试重新创建连接
// 返回: *gorm.DB - 数据库连接实例, error - 错误信息
func GetDB() (*gorm.DB, error) {
	// 检查连接是否已存在
	if db != nil {
		return db, nil
	}

	// 连接不存在时，创建新连接
	var err error
	db, err = connectDatabase()
	return db, err
}

// connectDatabase 内部函数：创建数据库连接并配置连接池
// 支持MySQL和PostgreSQL两种数据库
// 返回: *gorm.DB - 数据库连接实例, error - 错误信息
func connectDatabase() (*gorm.DB, error) {
	// 根据配置选择数据库驱动
	var dialector gorm.Dialector
	driver := config.Database.Driver
	dsn := config.Database.GetDSN()

	// 支持PostgreSQL和MySQL两种数据库
	if driver == "postgresql" || driver == "postgres" {
		dialector = postgres.Open(dsn)
	} else {
		// 默认使用MySQL
		dialector = mysql.New(mysql.Config{
			DSN:                       dsn,   // 数据源名称
			DefaultStringSize:         256,   // string类型字段的默认长度
			DisableDatetimePrecision:  true,  // 禁用datetime精度，MySQL 5.6之前的数据库不支持
			DontSupportRenameIndex:    true,  // 重命名索引时采用删除并新建的方式
			DontSupportRenameColumn:   true,  // 用`change`重命名列
			SkipInitializeWithVersion: false, // 根据当前MySQL版本自动配置
		})
	}

	// 打开数据库连接
	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := gormDB.DB()
	if err == nil {
		// 设置连接池参数
		sqlDB.SetMaxIdleConns(10)  // 最大空闲连接数
		sqlDB.SetMaxOpenConns(100) // 最大打开连接数
	}

	return gormDB, nil
}
