package dingding

import (
	"encoding/json"
	"fmt"
	"log"
)

// 从dingding中同步部门和用户数据
func RsyncDingDingData() (err error) {
	var (
		dingDepartments []*DingDepartment
		department      *Department
	)
	// 第1步：先同步所有部门
	if dingDepartments, err = ding.ListDepartment(); err != nil {
		log.Println("同步钉钉部门出错：", err.Error())
		return
	} else {
		// 第2步：保存部门信息
		for _, dingDepartment := range dingDepartments {
			// 实例化Department
			// log.Println(dingDepartment)

			department = &Department{}
			// 判断是否在数据库中: 根据名字判断
			db.FirstOrCreate(department, "name=? or ding_id=?", dingDepartment.Name, dingDepartment.ID)

			// 赋值
			department.Name = dingDepartment.Name
			department.DingID = dingDepartment.ID
			if dingData, err := json.Marshal(dingDepartment); err != nil {
				dingData = []byte{}
			} else {
				department.DingData = dingData
			}
			// 保存到数据库中
			db.Model(department).Update(department)
			//log.Println(department)

			// 第3步：同步部门用户
			msg := fmt.Sprintf("开始同步部门(%s:%d)用户", department.Name, department.DingID)
			log.Println(msg)
			if err = rsyncDepartmentUser(department); err != nil {
				log.Printf("同步部门(%s:%d)用户出错：%s\n", department.Name, department.DingID, err.Error())
			} else {
				msg := fmt.Sprintf("同步部门用户成功\n")
				log.Println(msg)
			}
		}
	}
	return
}

// 同步部门用户
func rsyncDepartmentUser(department *Department) (err error) {
	var (
		offSet   int
		pageSize int
		haveNext bool // 是否还有下一页
	)
	offSet = 0
	pageSize = 10
	haveNext = true

	for haveNext {
		if dingUsers, err := ding.GetDepartmentUserList(department.DingID, offSet, pageSize); err != nil {
			log.Println(err.Error())
			return err
		} else {
			if len(dingUsers) == pageSize {
				haveNext = true
			} else {
				haveNext = false
			}
			//log.Println(dingUsers)

			// 遍历用户，插入到数据库中
			for _, dingUser := range dingUsers {
				user := &User{}
				db.FirstOrCreate(user, "username = ? or ding_id = ?", dingUser.Name, dingUser.UserId)
				//	赋值
				user.Username = dingUser.Name
				user.DingID = dingUser.UserId
				user.Mobile = dingUser.Mobile
				if dingData, err := json.Marshal(dingUser); err != nil {
					dingData = []byte{}
				} else {
					user.DingData = dingData
				}
				// 保存到数据库中
				db.Model(user).Update(user)
				log.Println(user.Model.ID, user.Username, user.DingID, user.Mobile)

				//	保存关系：M2M
				db.Model(user).Association("Departments").Append(department)
			}
		}
	}
	return nil
}
