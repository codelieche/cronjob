package main

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	Name string `bson: "name"` // 名字
	Age  uint64 `bson: "age"`  // 年龄
	Desc string `bson: "desc"` // 描述
}

type DeleteByName struct {
	Name string `bson: "name"`
}

func main() {
	// 定义变量
	var (
		option *options.ClientOptions
		client *mongo.Client
		err    error

		database   *mongo.Database
		collection *mongo.Collection

		deleteResult *mongo.DeleteResult
	)

	log.SetFlags(log.Lshortfile | log.Ldate)

	//	配置文件
	option = &options.ClientOptions{
		Hosts: []string{"127.0.0.1:27017"},
		Auth: &options.Credential{
			Username: "root",
			Password: "password",
		},
	}

	//  连接客户端
	client, err = mongo.Connect(context.TODO(), option)
	if err != nil {
		log.Panic(err)
	}

	// 选择数据库
	database = client.Database("study")

	//	选择表
	collection = database.Collection("users")

	//  删除单条数据
	if deleteResult, err = collection.DeleteOne(context.TODO(), bson.D{{"age", 19}}); err != nil {
		log.Panic(err)
	} else {
		log.Println("删除的数据条数：", deleteResult.DeletedCount)
	}

	//  删除多条记录
	filter := &DeleteByName{Name: "user 2"}
	if deleteResult, err = collection.DeleteMany(context.TODO(), filter); err != nil {
		log.Panic(err)
	} else {
		log.Println("删除的数据条数：", deleteResult.DeletedCount)
	}

	log.Println("=== Done ===")

}
