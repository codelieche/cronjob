package main

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	Name string `bson: "name"` // 名字
	Age  uint64 `bson: "age"`  // 年龄
	Desc string `bson: "desc"` // 描述
}

func main() {
	// 定义变量
	var (
		option *options.ClientOptions
		client *mongo.Client
		err    error

		database       *mongo.Database
		collection     *mongo.Collection
		insertOneRsult *mongo.InsertOneResult
	)

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
	log.Println(client)

	log.Println(database.Name())

	//	插入单条记录
	//  选择表
	collection = database.Collection("users")

	//	实例化user
	var i uint64
	for {
		i += 1
		if i > 10 {
			break
		}
		user := User{
			Name: fmt.Sprintf("user %d", i),
			Age:  i + 18,
			Desc: "user desc",
		}
		// 插入对象到mongodb中
		if insertOneRsult, err = collection.InsertOne(context.TODO(), user); err != nil {
			log.Println(err.Error())
			continue
		} else {
			log.Println("插入数据成功：", insertOneRsult.InsertedID)
		}
	}

	log.Println("=== Done ===")

}
