package main

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson/primitive"

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

		database        *mongo.Database
		collection      *mongo.Collection
		insertManyRsult *mongo.InsertManyResult
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
	log.Println(client)

	log.Println(database.Name())

	//	选择表
	collection = database.Collection("users")

	//	实例化user

	// 插入多条记录
	// 注意：这里定义的interface的数组
	var users []interface{}

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
		users = append(users, user)
	}

	// log.Println(users)

	// 插入数据
	if insertManyRsult, err = collection.InsertMany(context.TODO(), users); err != nil {
		log.Panic(err.Error())
	} else {
		for _, item := range insertManyRsult.InsertedIDs {
			objectID := item.(primitive.ObjectID)
			log.Println(objectID.Hex())
		}
	}

	log.Println("=== Done ===")

}
