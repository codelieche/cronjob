package common

import (
	"log"
	"testing"
)

func TestInsertCategory(t *testing.T) {
	// 实例化Category
	//cmd := CategoryCommand{
	//	Check:    "which bash",
	//	Setup:    "echo `date`",
	//	TearDown: "echo `date`",
	//}
	c := Category{
		Name:        "default",
		IsActive:    true,
		CheckCmd:    "which bash",
		SetupCmd:    "echo `date`",
		TearDownCmd: "echo `date`",
	}

	//	实例化etcdManager
	if etcdManager, err := NewEtcdManager(Config.Master.Etcd); err != nil {
		t.Error(err)
		return
	} else {
		if prevCategory, err := etcdManager.SaveCategory(&c, true); err != nil {
			t.Error(err)
		} else {
			log.Println(prevCategory)
		}
	}
}

// 测试获取分类
func TestGetCategory(t *testing.T) {

	//	实例化etcdManager
	if etcdManager, err := NewEtcdManager(Config.Master.Etcd); err != nil {
		t.Error(err)
		return
	} else {
		if c, err := etcdManager.GetCategory("default"); err != nil {
			t.Error(err)
		} else {
			log.Println(c)
		}
	}
}

// 测试获取分类列表
func TestListCategories(t *testing.T) {

	//	实例化etcdManager
	if etcdManager, err := NewEtcdManager(Config.Master.Etcd); err != nil {
		t.Error(err)
		return
	} else {
		if cArr, err := etcdManager.ListCategories(); err != nil {
			t.Error(err)
		} else {
			for _, c := range cArr {
				log.Println(c)
			}
		}
	}
}
