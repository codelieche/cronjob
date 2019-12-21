package datasources

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client     *mongo.Client
	Database   *mongo.Database
	Collection *mongo.Collection
}

var mongoDB *MongoDB

func connectMongoDB(mongoConfig *common.MongoConfig) {
	// 定义变量
	var (
		option *options.ClientOptions
		client *mongo.Client
		err    error

		database   *mongo.Database
		collection *mongo.Collection
	)

	// 配置文件
	timeOut := time.Duration(10) * time.Second
	option = &options.ClientOptions{
		AppName: nil,
		Auth: &options.Credential{
			Username: mongoConfig.User,
			Password: mongoConfig.Password,
		},
		ConnectTimeout: &timeOut,
		Hosts:          mongoConfig.Hosts,
	}

	// 连接客户端
	if client, err = mongo.Connect(context.TODO(), option); err != nil {
		log.Println("连接MongoDB数据库出错：", err)
		os.Exit(1)
	}

	// 选择数据库
	if mongoConfig.Database == "" {
		mongoConfig.Database = "cronjob"
	}

	database = client.Database(mongoConfig.Database)
	collection = database.Collection("logs")

	// 实例化MongoDB
	mongoDB = &MongoDB{
		Client:     client,
		Database:   database,
		Collection: collection,
	}
}

func GetMongoDB() *MongoDB {
	if mongoDB != nil {
		return mongoDB
	} else {
		// 1. 获取配置
		config := common.Config
		connectMongoDB(config.Master.Mongo)
		return mongoDB
	}
}
