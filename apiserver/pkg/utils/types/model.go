package types

import (
	"errors"
	"reflect"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/tools"
	"gorm.io/gorm"
)

// ModelWithSoftDelete 接口定义了软删除模型需要实现的方法
// 子结构体需要实现此接口以使用扩展功能
// type ModelWithSoftDelete interface {
// 	GetDeleteTasks() []string
// 	GetSecretFields() []string
// 	GetDeleteUpdateFields() []string
// 	GetDeleteTimeFieldName() string
// }

type BaseModel struct {
	ID        uint           `gorm:"primaryKey" json:"id,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at,omitempty"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Deleted   *bool          `gorm:"type:boolean;default:false" json:"deleted" form:"deleted"`
}

// Strftime 格式化当前的时间戳，删除资源时会用到
func (m *BaseModel) Strftime(format string) string {
	if format == "" {
		format = "20060102150405"
	}
	return time.Now().Format(format)
}

func (m *BaseModel) GetDeleteUpdateFields() []string {
	return []string{}
}

// 全局加密密钥配置
var (
	// EncryptionKey 用于字段加密和解密的密钥
	// 实际应用中应该从配置文件或环境变量中读取
	EncryptionKey = "default-encryption-key-for-model"
)

// SetEncryptionKey 设置全局加密密钥
func SetEncryptionKey(key string) {
	EncryptionKey = key
}

// GetEncryptionKey 获取全局加密密钥
func GetEncryptionKey() string {
	return EncryptionKey
}

// GetDecryptValue 获取解密后的字段值
func (m *BaseModel) GetDecryptValue(fieldName string) (string, error) {
	// 使用反射获取结构体字段
	v := reflect.ValueOf(m).Elem()
	fieldValue := v.FieldByName(fieldName)

	if !fieldValue.IsValid() || !fieldValue.CanInterface() {
		return "", errors.New("字段不存在或无法访问")
	}

	// 检查字段是否为字符串类型
	fieldStr, ok := fieldValue.Interface().(string)
	if !ok {
		return "", errors.New("字段不是字符串类型")
	}

	// 如果字段为空，直接返回
	if fieldStr == "" {
		return "", nil
	}

	// 创建解密实例（使用全局加密密钥）
	crypto := tools.NewCryptography(GetEncryptionKey())

	// 检查字符串是否为加密格式
	isEncrypted, decrypted := crypto.CheckCanDecrypt(fieldStr)
	if !isEncrypted {
		// 如果不是加密格式，直接返回原始字符串
		return fieldStr, nil
	}

	// 执行解密操作
	// 注意：由于我们在CheckCanDecrypt中已经获取了解密后的值，这里可以直接返回
	return decrypted, nil
}

// ExecuteTask 通过反射执行函数
func (m *BaseModel) ExecuteTask(tasks []string) {
	// 获取到需要删除的任务
	if len(tasks) == 0 {
		return
	}
	// 通过反射判断是否有这个函数，有就执行这个task
	for _, task := range tasks {
		// 检查是否有这个方法
		method := reflect.ValueOf(m).MethodByName(task)
		if !method.IsValid() {
			continue
		}
		// 调用方法
		method.Call([]reflect.Value{})
	}
}

// BeforeDelete 删除前设置deleted字段为True
// 同时执行删除操作的额外处理
func (m *BaseModel) BeforeDelete(tx *gorm.DB) (err error) {
	// 设置Deleted字段为true
	trueValue := true
	m.Deleted = &trueValue

	return nil
}

// AfterDelete 钩子函数，在删除后执行
func (m *BaseModel) AfterDelete(tx *gorm.DB) (err error) {
	// 这里可以添加删除后的处理逻辑
	return
}

// BeforeSave 保存前钩子，处理加密字段
func (m *BaseModel) BeforeSave(tx *gorm.DB) (err error) {
	return
}
