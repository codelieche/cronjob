package repositories

import (
	"errors"
	"log"
	"strings"

	"github.com/codelieche/cronjob/tools/dingding/common"

	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/jinzhu/gorm"
)

// 用户Repository接口
type UserRepository interface {
	// 根据ID获取到用户
	GetById(id string) (user *datamodels.User, err error)
	// 根据名字获取用户
	GetByName(name string) (user *datamodels.User, err error)
	// 通过ID或者名字获取到用户
	GetByIdOrName(idOrName string) (user *datamodels.User, err error)
	// 根据手机号获取用户
	GetByMobile(mobile string) (user *datamodels.User, err error)
	// 获取用户列表
	List(offset int, limit int) (users []*datamodels.User, err error)
	// 获取用户的所有部门
	GetUserDepartments(user *datamodels.User) (departments []*datamodels.Department, err error)
	//	获取用户的消息列表
	GetUserMessagesList(user *datamodels.User, offset int, limit int) (messages []*datamodels.Message, err error)
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// 用户Repository
type userRepository struct {
	db *gorm.DB
}

// 根据用户的ID获取到用户
// 有可能是数字ID也可能是钉钉ID(字符)
func (r *userRepository) GetById(userId string) (user *datamodels.User, err error) {
	userId = strings.TrimSpace(userId)
	if userId == "" {
		err = errors.New("传入的ID不可为空")
		return nil, err
	}

	user = &datamodels.User{}

	r.db.First(user, "id=? or ding_id=?", userId, userId)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 根据用户名字获取用户
func (r *userRepository) GetByName(name string) (user *datamodels.User, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		err = errors.New("传入的名字不可为空")
		return nil, err
	}
	user = &datamodels.User{}
	log.Println(name)

	r.db.First(user, "username = ?", name)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 通过id或者用户名获取到用户
func (r *userRepository) GetByIdOrName(idOrName string) (user *datamodels.User, err error) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		err = errors.New("传入的ID/Name不可为空")
		return nil, err
	}

	user = &datamodels.User{}

	r.db.First(user, "id=? or ding_id=? or username=?", idOrName, idOrName, idOrName)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 根据手机号获取用户
func (r *userRepository) GetByMobile(mobile string) (user *datamodels.User, err error) {
	mobile = strings.TrimSpace(mobile)
	if mobile == "" {
		err = errors.New("传入的手机号不可为空")
		return nil, err
	}
	user = &datamodels.User{}

	r.db.First(user, "mobile = ?", mobile)
	if user.ID > 0 {
		// 获取到了用户
		return user, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 获取用户列表
func (r *userRepository) List(offset int, limit int) (users []*datamodels.User, err error) {
	query := r.db.Model(&datamodels.User{}).Offset(offset).Limit(limit).Find(&users)
	if query.Error != nil {
		log.Println(err.Error())
		return nil, err
	} else {
		return users, err
	}
}

// 获取用户的部门列表
func (r *userRepository) GetUserDepartments(user *datamodels.User) (departments []*datamodels.Department, err error) {
	query := r.db.Model(user).Related(&departments, "Departments")
	if query.Error != nil {
		return nil, query.Error
	} else {
		return departments, nil
	}
}

// 获取用户消息列表
func (r *userRepository) GetUserMessagesList(user *datamodels.User, offset int, limit int) (messages []*datamodels.Message, err error) {
	query := r.db.Model(user).Offset(offset).Limit(limit).Preload("Users").Related(&messages, "Messages")
	if query.Error != nil {
		return nil, query.Error
	} else {
		return messages, nil
	}
}
