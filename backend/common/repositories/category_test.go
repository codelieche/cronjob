package repositories

import (
	"log"
	"testing"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

func initCategoryRepository() *categoryRepository {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := &categoryRepository{
		db:         db,
		etcd:       etcd,
		infoFields: nil,
	}
	return r
}

// 测试获取分类列表
func TestCategoryRepository_List(t *testing.T) {
	// 1. get db
	db := datasources.GetDb()
	etcd := datasources.GetEtcd()

	// 2. init repository
	r := NewCategoryRepository(db, etcd)

	// 3. 测试获取数据
	haveNext := true
	offset := 0
	limit := 5
	for haveNext {
		if categories, err := r.List(offset, limit); err != nil {
			t.Error(err.Error())
		} else {
			// 判断是否还有下一页
			if len(categories) == limit && limit > 0 {
				haveNext = true
				offset += limit
			} else {
				haveNext = false
			}

			// 输出结果
			for _, category := range categories {
				log.Println(category.ID, category.Name, category.SetupCmd, category.Description, category.IsActive)
			}
		}
	}
}

// 测试从etcd中获取所有分类
func TestCategoryRepository_List2(t *testing.T) {
	r := initCategoryRepository()

	// 从etcd中获取分类列表
	if categories, err := r.listCategoriesFromEtcd(); err != nil {
		t.Error(err.Error())
	} else {
		for _, category := range categories {
			log.Println(category.ID, category.Name, category.IsActive, category.Description)
		}
	}

}
