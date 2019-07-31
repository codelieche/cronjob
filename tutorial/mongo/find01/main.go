package main

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	Name string `bson: "name"` // 名字
	Age  uint64 `bson: "age"`  // 年龄
	Desc string `bson: "desc"` // 描述
}

type FindByName struct {
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

		cursor *mongo.Cursor
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

	//  查找数据

	//filter := bson.D{{"name", "user 9"}}
	//filter := bson.D{}

	filter := &FindByName{Name: "user 1"}

	//options := &options.FindOptions{
	//	Max: 2,
	//}

	if cursor, err = collection.Find(context.TODO(), filter); err != nil {
		log.Panic(err)
	} else {
		// 记得关闭cursor
		defer cursor.Close(context.TODO())

		// 遍历输出查询到的结果
		for cursor.Next(context.TODO()) {
			var u User
			cursor.Decode(&u)
			log.Printf("名字：%s, 年龄：%d", u.Name, u.Age)
		}
	}

	log.Println("=== Done ===")

}
