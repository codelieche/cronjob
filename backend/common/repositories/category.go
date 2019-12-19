package repositories

import (
	"errors"
	"log"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/datasources"
	"github.com/jinzhu/gorm"
)

type CategoryRepository interface {
	// 保存Category
	Save(category *datamodels.Category) (*datamodels.Category, error)
	// 获取Category的列表
	List(offset int, limit int) ([]*datamodels.Category, error)
	// 获取Category信息
	Get(id int64) (*datamodels.Category, error)
	// 根据Category的Name获取信息
	GetByName(name string) (*datamodels.Category, error)
	// 根据ID或者Name获取Category
	GetByIdOrName(idOrName string) (*datamodels.Category, error)
	// 删除Category
	Delete(category *datamodels.Category) (err error)
	// Update
	Update(category *datamodels.Category, fields map[string]interface{}) (*datamodels.Category, error)
	// Update By Id
	UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Category, error)
}

// 实例化Category Repository
func NewCategoryRepository(db *gorm.DB, etcd *datasources.Etcd) CategoryRepository {
	return &categoryRepository{
		db:   db,
		etcd: etcd,
		infoFields: []string{
			"id", "created_at", "updated_at", "deleted_at",
			"etcd_key", "name", "description", "setup_cmd", "check_cmd", "tear_down_cmd", "is_active"},
	}
}

type categoryRepository struct {
	db         *gorm.DB
	etcd       *datasources.Etcd
	infoFields []string // 基本信息字段
}

// 保存Category
func (r *categoryRepository) Save(category *datamodels.Category) (*datamodels.Category, error) {
	if category.ID > 0 {
		// 是更新操作
		if err := r.db.Model(&datamodels.Category{}).Update(category).Error; err != nil {
			return nil, err
		} else {
			// 需要更新一下etcd中的数据
			if prevCategory, err := r.saveCategoryToEtcd(category, false); err != nil {
				// 保存去etcd出错
				// 当不存在的时候，就需要重新创建一下
				if err == common.NotFountError {
					// 不存在etcd中，我们需要创建一下
					if _, err = r.saveCategoryToEtcd(category, true); err != nil {
						log.Println("创建成功了，但是保存到etcd的时候，出错了", err.Error())
					}
				} else {
					log.Println("保存到mysql成功了，但是保存到etcd的时候，出错了", err.Error())
				}

			} else {
				log.Println(prevCategory)
			}
			return category, nil
		}
	} else {
		// 是创建操作
		if err := r.db.Create(category).Error; err != nil {
			return nil, err
		} else {
			// 需要插入到etcd中
			if _, err := r.saveCategoryToEtcd(category, true); err != nil {
				log.Println("创建成功了，但是保存到etcd的时候，出错了", err.Error())
			} else {
				//log.Println("保存到etcd成功")
			}
			return category, nil
		}
	}
}

// 获取Category的列表
func (r *categoryRepository) List(offset int, limit int) (categories []*datamodels.Category, err error) {
	query := r.db.Model(&datamodels.Category{}).Select(r.infoFields).Offset(offset).Limit(limit).Find(&categories)
	if query.Error != nil {
		return nil, query.Error
	} else {
		return categories, nil
	}
}

// 根据ID获取Category
func (r *categoryRepository) Get(id int64) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "id = ?", id)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}

// 根据Name获取Category
func (r *categoryRepository) GetByName(name string) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "name = ?", name)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}

// 根据ID或者name获取Category
func (r *categoryRepository) GetByIdOrName(idOrName string) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "id = ? or name = ?", idOrName, idOrName)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}

// 删除分类
func (r *categoryRepository) Delete(category *datamodels.Category) (err error) {
	if category.IsActive {
		category.IsActive = false
		log.Println(category)
		if category, err = r.Update(category, map[string]interface{}{"IsActive": false}); err != nil {
			return err
		} else {
			return nil
		}
	} else {
		return nil
	}
}

// 更新分类
func (r *categoryRepository) Update(category *datamodels.Category, fields map[string]interface{}) (*datamodels.Category, error) {
	// 判断ID：
	// 如果传入的是0，那么会更新全部
	// 如果fields中传入了ID，那么会更新ID是它的对象
	// 推荐加一个limit(1), 确保只更新一条数据
	if category.ID <= 0 {
		err := errors.New("传入ID为0，会更新全部数据")
		return nil, err
	}

	// 丢弃ID/Id/iD
	idKeys := []string{"ID", "id", "Id", "iD"}
	for _, k := range idKeys {
		if _, exist := fields[k]; exist {
			delete(fields, k)
		}
	}

	// 更新操作
	if err := r.db.Model(category).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		return category, nil
	}
}

// 更新分类
func (r *categoryRepository) UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Category, error) {
	// 判断ID
	if id <= 0 {
		err := errors.New("传入的ID为0，会更新全部数据")
		return nil, err
	}

	// 更新操作
	if err := r.db.Model(&datamodels.Category{}).Where("id = ?", id).Limit(1).Update(fields).Error; err != nil {
		return nil, err
	} else {
		// 返回获取到的对象
		return r.Get(id)
	}
}
