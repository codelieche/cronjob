package services

import (
	"github.com/codelieche/cronjob/backend/common/datamodels"
	"github.com/codelieche/cronjob/backend/common/repositories"
)

// Category Service Interface
type CategoryService interface {
	// 创建分类
	Create(category *datamodels.Category) (*datamodels.Category, error)
	// 保存分类
	Save(category *datamodels.Category) (*datamodels.Category, error)
	// 根据ID获取分类
	GetByID(id int64) (category *datamodels.Category, err error)
	// 根据Name获取分类
	GetByName(name string) (category *datamodels.Category, err error)
	// 根据ID或者Name获取分类
	GetByIdORName(idOrName string) (category *datamodels.Category, err error)
	// 获取分类的列表
	List(offset int, limit int) (categories []*datamodels.Category, err error)
	// 删除分类
	Delete(category *datamodels.Category) (err error)
	// 更新分类
	Update(category *datamodels.Category, fields map[string]interface{}) (*datamodels.Category, error)
	// 更新分类
	UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Category, error)
}

// 实例化Category Service
func NewCategoryService(repo repositories.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

// Category Service
type categoryService struct {
	repo repositories.CategoryRepository
}

// 创建分类
func (s categoryService) Create(category *datamodels.Category) (*datamodels.Category, error) {
	return s.repo.Save(category)
}

// 保存分类
func (s categoryService) Save(category *datamodels.Category) (*datamodels.Category, error) {
	return s.repo.Save(category)
}

// 根据ID获取分类
func (s categoryService) GetByID(id int64) (category *datamodels.Category, err error) {
	return s.repo.Get(id)
}

// 根据Name获取分类
func (s categoryService) GetByName(name string) (category *datamodels.Category, err error) {
	return s.repo.GetByName(name)
}

// 根据ID或者Name获取分类
func (s categoryService) GetByIdORName(idOrName string) (category *datamodels.Category, err error) {
	return s.repo.GetByIdOrName(idOrName)
}

// 获取分类的列表
func (s categoryService) List(offset int, limit int) (categories []*datamodels.Category, err error) {
	return s.repo.List(offset, limit)
}

// 删除分类
func (s categoryService) Delete(category *datamodels.Category) (err error) {
	return s.repo.Delete(category)
}

// 更新分类
func (s categoryService) Update(category *datamodels.Category, fields map[string]interface{}) (*datamodels.Category, error) {
	return s.repo.Update(category, fields)
}

// 更新分类By ID
func (s categoryService) UpdateByID(id int64, fields map[string]interface{}) (*datamodels.Category, error) {
	return s.repo.UpdateByID(id, fields)
}
