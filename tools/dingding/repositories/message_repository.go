package repositories

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/codelieche/cronjob/tools/dingding/datasource"

	"github.com/levigross/grequests"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/jinzhu/gorm"
)

// 消息Repository接口
type MessageRepository interface {
	//	获取消息
	GetById(id int64) (message *datamodels.Message, err error)
	// 获取消息列表
	List(offset int, limit int) (messages []*datamodels.Message, err error)
	// 发送工作消息
	SendWorkerMessage(workMessage *common.WorkerMessage, message *datamodels.Message) (success bool, err error)
}

// 消息 Repository
type messageRepository struct {
	db   *gorm.DB
	ding *common.DingDing
}

// 实例化Message Repository
func NewMessageRepository(db *gorm.DB, ding *common.DingDing) MessageRepository {
	return &messageRepository{db: db, ding: ding}
}

// 通过ID获取消息
func (r *messageRepository) GetById(id int64) (message *datamodels.Message, err error) {
	if id == 0 {
		err = errors.New("传入的ID不可为空")
		return nil, err
	}

	message = &datamodels.Message{}

	r.db.First(message, "id=?", id)
	if message.ID > 0 {
		// 获取到了用户
		return message, nil
	} else {
		// 未获取到
		return nil, common.NotFountError
	}
}

// 获取消息列表
func (r *messageRepository) List(offset int, limit int) (messages []*datamodels.Message, err error) {
	query := r.db.Model(&datamodels.Message{}).Offset(offset).Limit(limit).Find(&messages)
	if query.Error != nil {
		return nil, err
	} else {
		return messages, err
	}
}

// 发送工作消息
func (r *messageRepository) SendWorkerMessage(workMessage *common.WorkerMessage, message *datamodels.Message) (success bool, err error) {
	// 发送工作消息
	var (
		accessToken string
		url         string
		ro          *grequests.RequestOptions
		response    *grequests.Response
		//data        []byte
		apiResponse *common.ApiResponse
	)
	if message != nil {
		defer datasource.SaveMessage(message)
	}
	// 设置AgentID
	workMessage.AgentID = r.ding.AgentId

	// 对接收的人做校验
	if workMessage.ToAllUser {
		// 如果设置了所有人，注意提醒
		log.Println("本次执行发送消息为所有人！")
	} else {
		if workMessage.UseridList == "" && workMessage.DeptIdList == "" {
			msg := fmt.Sprintf("传入的用户列表、部门列表为空")
			err = errors.New(msg)
			return false, err
		}
	}

	// 准备数据
	if accessToken, err = r.ding.GetAccessToken(); err != nil {
		log.Println(err.Error())
		return false, err
	}

	url = fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/"+
		"corpconversation/asyncsend_v2?access_token=%s", accessToken)
	//if data, err = json.Marshal(workMessage); err != nil {
	//	log.Println(err.Error())
	//	return false, err
	//}

	//if data, err := json.Marshal(workMessage); err != nil {
	//	log.Println(err.Error())
	//	return false, err
	//} else {
	//	log.Println(url)
	//	log.Println(string(data))
	//}

	ro = &grequests.RequestOptions{
		Data:        nil,
		Params:      nil,
		JSON:        workMessage,
		Headers:     map[string]string{"Content-Type": "application/json"},
		UserAgent:   "",
		RequestBody: nil,
	}

	// 开始发送消息
	if response, err = grequests.Post(url, ro); err != nil {
		log.Println(err.Error())
		return false, err
	} else {
		// 开始处理结果
		apiResponse = &common.ApiResponse{}
		if err = json.Unmarshal(response.Bytes(), apiResponse); err != nil {
			log.Println(err.Error())
			if message != nil {
				message.Success = false
			}
			return false, err
		} else {
			// 对结果进行判断
			if apiResponse.Errcode != 0 {
				if message != nil {
					message.Success = false
					message.DingResponse = response.Bytes()
					//message.Save()
				}

				msg := fmt.Sprintf("获取数据出错，错误代码:%d(%s)", apiResponse.Errcode, apiResponse.Errmsg)
				err = errors.New(msg)
				return false, err
			} else {
				// 发送消息成功
				if message != nil {
					// 保存发送消息的记录
					message.Success = true
					message.DingResponse = response.Bytes()
					//message.Save()
				}

				return true, nil
			}
		}
	}
}
