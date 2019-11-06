package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type User struct {
	gorm.Model
	Name      string
	Age       int
	Describle string
}

func main() {
	// 连接数据库
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.Println(os.Getenv("PWD"))
	db, err := gorm.Open("sqlite3", "sqlite3.db")
	if err != nil {
		log.Println(err.Error())
		return
	}
	// 关闭db
	defer db.Close()

	//	Migrate the schema
	db.AutoMigrate(&User{})

	// 创建user
	i := 0
	for {
		i++
		name := fmt.Sprintf("User:%d", i)
		age := 18 + i

		user := User{
			Name:      name,
			Age:       age,
			Describle: name,
		}

		// 插入到数据库
		db.Create(&user)

		// 只插入20条数据
		if i > 20 {
			break
		}
	}

	//	获取user
	var user User
	// 获取id是10的用户
	db.First(&user, 10)
	log.Println(user)

	// 根据名字查询
	var u2 User
	db.First(&u2, "name=?", "User:18")
	log.Println(u2)

	// 更新上面查询的用户的描述
	db.Model(&user).Update("Describle", "新的描述内容")

	//	删除
	var userForDelete User
	db.First(&userForDelete, 17)
	log.Println(userForDelete)
	if userForDelete.ID > 0 {
		db.Delete(&userForDelete)
	}

	// 查询用户:设置offset和Limit
	var users []*User
	query := db.Offset(10).Limit(2).Find(&users)
	if query.Error != nil {
		log.Println(query.Error)
		return
	} else {
		for i, u := range users {
			log.Println(i, u)
		}
	}

}
