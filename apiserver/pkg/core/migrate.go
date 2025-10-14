package core

import "gorm.io/gorm"

// AutoMigrate 执行数据库自动迁移
// 该函数会根据定义的模型结构体自动创建或更新数据库表结构
// 参数:
//
//	db: GORM数据库连接实例（注意：此参数实际上未被使用，函数内部会重新获取数据库连接）
//
// 返回值:
//
//	error: 如果迁移过程中出现错误则返回错误信息，成功则返回nil
func AutoMigrate(db *gorm.DB) error {
	// 获取数据库连接
	// 注意：这里重新获取数据库连接，传入的db参数实际未被使用
	if db, err := GetDB(); err != nil {
		// 如果获取数据库连接失败，返回错误
		return err
	} else {
		// 执行自动迁移
		// AutoMigrate会检查模型结构体与数据库表的差异，并自动创建缺失的表和字段
		if err := db.AutoMigrate(
			// 核心业务表
			&Worker{},
			&Category{},
			&CronJob{},
			&Task{},
			&TaskLog{},

			// 凭证管理表
			&Credential{}, // 凭证信息

			// 🔥 统计数据表（用于性能优化）
			&TaskStatsDaily{},    // 任务每日统计
			&CronjobStatsDaily{}, // CronJob每日统计
			&WorkerStatsDaily{},  // Worker每日统计
			&TaskStatsHourly{},   // 任务每小时统计（可选）
		); err != nil {
			// 如果迁移过程中出现错误，返回错误信息
			return err
		}
		// 如果迁移成功，返回nil
		return nil
	}
}
