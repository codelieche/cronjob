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
	Name      string `gorm:"type:vachar(40);qunique_index;NOT NULL"`
	Age       int    `gorm:"default: 18;type:int"`
	Describle string `gorm:"default: 描述内容;size:512"`
}

func insertUsersData(filename string) {
	db, err := gorm.Open("sqlite3", filename)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Migrate the schema
	db.AutoMigrate(&User{})
	// 插入100条用户数据
	i := 0
	for {
		i++
		name := fmt.Sprintf("User:%d", i)
		age := 18 + i
		describle := "描述内容:" + name
		u := User{
			Name:      name,
			Age:       age,
			Describle: describle,
		}
		db.Create(&u)
		// 跳出循环
		if i >= 100 {
			break
		}
	}
}

// 准备数据
// 往user表中插入100条数据
func GenerateData() error {
	//	判断文件是否存在
	fmt.Println(os.Getenv("PWD"))
	filename := "sqlite3.db"
	if info, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			// 如果是不存在，才做创建数据的操作
			insertUsersData(filename)
		} else {
			// 其它位置错误
			return err
		}
	} else {
		// 文件已经存在
		log.Println(info)
		return nil
	}

	return nil
}

// 列出数据
func listUsers(filename string) {
	db, err := gorm.Open("sqlite3", filename)
	if err != nil {
		log.Println(err.Error())
		return
	}
	var users []*User
	query := db.Where("id > ?", 90).Limit(5).Find(&users)
	if query.Error != nil {
		log.Println(query.Error.Error())
		return
	} else {
		// 查询ok
		for i, u := range users {
			log.Println(i, u.Name, u.Age, u.Describle)
		}
	}
}

func main() {
	if err := GenerateData(); err != nil {
		panic(err)
	} else {
		log.Println("Generate Data Done")
	}
	// 列出数据
	filename := "sqlite3.db"
	listUsers(filename)
}
