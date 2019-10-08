package worker

import (
	"context"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 日志处理的接口
type LogHandler interface {
	ConsumeLogsLoop()                              // 消费日志循环函数
	AddLog(executeLog *common.JobExecuteLog) error // 添加日志
}

// 日志处理器--mongo
type MongoLogHandler struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	logChan    chan *common.JobExecuteLog // 日志channel
}

// 消费日志循环
func (logHandler *MongoLogHandler) ConsumeLogsLoop() {
	var (
		executeLog      *common.JobExecuteLog
		insertOneResult *mongo.InsertOneResult
		err             error
	)

	for {
		select {
		case executeLog = <-logHandler.logChan:
			if executeLog == nil {
				goto END
			}

			log.Println(executeLog)

			// 插入对象到mongodb中
			if insertOneResult, err = logHandler.collection.InsertOne(context.TODO(), executeLog); err != nil {
				log.Println(err.Error())
				continue
			} else {
				log.Println("插入数据成功：", insertOneResult.InsertedID)
			}
		}
	}
END:
	log.Println("跳出logHandler.ConsumeLogsLoop()")
	return
}

// 保存日志操作
func (logHandler *MongoLogHandler) AddLog(executeLog *common.JobExecuteLog) (err error) {

	// 把新的日志加入到channel中
	logHandler.logChan <- executeLog
	return
}

func NewMongoLogHandler() (logHandler *MongoLogHandler, err error) {
	var (
		option         *options.ClientOptions
		database       *mongo.Database
		collection     *mongo.Collection
		client         *mongo.Client
		connectTimeout time.Duration
	)

	//	配置文件
	connectTimeout = 10 * time.Second
	option = &options.ClientOptions{
		Hosts: []string{"127.0.0.1:27017"},
		Auth: &options.Credential{
			Username: "root",
			Password: "happydb",
		},
		ConnectTimeout: &connectTimeout,
	}

	if client, err = mongo.Connect(context.TODO(), option); err != nil {
		return
	}

	// 选择数据库
	database = client.Database("cronjob")

	//log.Println(client)
	//log.Println(database.Name())

	//	插入单条记录
	//  选择表
	collection = database.Collection("logs")

	logHandler = &MongoLogHandler{
		client:     client,
		database:   database,
		collection: collection,
		logChan:    make(chan *common.JobExecuteLog, 1000),
	}
	return
}
