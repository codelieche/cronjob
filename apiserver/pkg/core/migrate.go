package core

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	if db, err := GetDB(); err != nil {
		return err
	} else {
		if err := db.AutoMigrate(
			&User{},
			&Worker{},
			&Category{},
			&CronJob{},
			&Task{},
			&TaskLog{},
		); err != nil {
			return err
		}
		return nil
	}
}
