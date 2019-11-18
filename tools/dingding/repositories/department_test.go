package repositories

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/datasource"
)

// 测试根据部门ID获取部门相关信息
func TestDepartmentRespository_GetById(t *testing.T) {
	var departmentId int64 = 2
	// departmentId := 118434421
	var (
		department *datamodels.Department
		err        error
	)

	db := datasource.DB
	departmentRepository := NewDepartmentRepository(db)

	// 通过ID获取用户
	if department, err = departmentRepository.GetById(departmentId); err != nil {
		t.Error(err)
		return
	} else {
		log.Println(department.DingID, department.Name)
		// 获取用户的部门
		if users, err := departmentRepository.GetDepartmentUsers(department); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出用户
			log.Println("获取到用户个数：", len(users))
			for _, user := range users {
				log.Println(user.Username, user.DingID, user.Mobile)
			}
		}
	}
}

// 测试根据部门名字获取部门
func TestDepartmentRespository_GetByName(t *testing.T) {
	name := "技术部"
	// departmentId := "118434421"

	var (
		department *datamodels.Department
		err        error
	)

	db := datasource.DB
	r := NewDepartmentRepository(db)

	// 通过ID获取用户
	if department, err = r.GetByName(name); err != nil {
		t.Error(err)
		return
	} else {
		log.Println(department.DingID, department.Name)
		// 获取部门的用户
		if users, err := r.GetDepartmentUsers(department); err != nil {
			t.Error(err.Error())
			return
		} else {
			// 输出用户
			log.Println("获取到用户个数：", len(users))
			for _, user := range users {
				log.Println(user.ID, user.Username, user.DingID, user.Position, user.Mobile)
			}
		}
	}
}
