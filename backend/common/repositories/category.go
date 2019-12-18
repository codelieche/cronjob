package repositories

import (
	"github.com/codelieche/cronjob/backend/common"
	"github.com/codelieche/cronjob/backend/common/datamodels"
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
}

// 实例化Category Repository
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{
		db:         db,
		infoFields: []string{"id", "etcd_key", "name", "description", "setup_cmd", "check_cmd", "tear_down_cmd", "is_active"},
	}
}

type categoryRepository struct {
	db         *gorm.DB
	infoFields []string // 基本信息字段
}

// 保存Category
func (r categoryRepository) Save(category *datamodels.Category) (*datamodels.Category, error) {
	if category.ID > 0 {
		// 是更新操作
		if err := r.db.Model(&datamodels.Category{}).Update(category).Error; err != nil {
			return nil, err
		} else {
			return category, nil
		}
	} else {
		// 是创建操作
		if err := r.db.Create(category).Error; err != nil {
			return nil, err
		} else {
			return category, nil
		}
	}
}

// 获取Category的列表
func (r categoryRepository) List(offset int, limit int) (categories []*datamodels.Category, err error) {
	query := r.db.Model(&datamodels.Category{}).Select(r.infoFields).Offset(offset).Limit(limit).Find(&categories)
	if query.Error != nil {
		return nil, query.Error
	} else {
		return categories, nil
	}
}

// 根据ID获取Category
func (r categoryRepository) Get(id int64) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "id = ?", id)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}

// 根据Name获取Category
func (r categoryRepository) GetByName(name string) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "name = ?", name)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}

// 根据ID或者name获取Category
func (r categoryRepository) GetByIdOrName(idOrName string) (category *datamodels.Category, err error) {
	category = &datamodels.Category{}
	r.db.Select(r.infoFields).First(category, "id = ? or name = ?", idOrName, idOrName)
	if category.ID > 0 {
		return category, nil
	} else {
		return nil, common.NotFountError
	}
}
