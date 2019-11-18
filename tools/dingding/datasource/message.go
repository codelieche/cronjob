package datasource

import "github.com/codelieche/cronjob/tools/dingding/datamodels"

func SaveMessage(msg *datamodels.Message) {
	//	保存消息到数据库中
	if msg.ID == 0 {
		// 新创建
		DB.Model(&datamodels.Message{}).Create(msg)
	} else {
		DB.Model(&datamodels.Message{}).Update(msg)
	}
}
