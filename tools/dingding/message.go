package dingding

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/levigross/grequests"
)

// 发送工作通知消息
// Method： POST
// URL：https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=ACCESS_TOKEN
// 参数：
//
func (ding *DingDing) SendWorkerMessage(workMessage *WorkerMessage) (success bool, err error) {
	// 发送工作消息
	var (
		accessToken string
		url         string
		ro          *grequests.RequestOptions
		response    *grequests.Response
		//data        []byte
		apiResponse *ApiResponse
	)
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
	if accessToken, err = ding.GetAccessToken(); err != nil {
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
		apiResponse = &ApiResponse{}
		if err = json.Unmarshal(response.Bytes(), apiResponse); err != nil {
			log.Println(err.Error())
			return false, err
		} else {
			// 对结果进行判断
			if apiResponse.Errcode != 0 {
				msg := fmt.Sprintf("获取数据出错，错误代码:%d(%s)", apiResponse.Errcode, apiResponse.Errmsg)
				err = errors.New(msg)
				return false, err
			} else {
				// 发送消息成功
				return true, nil
			}
		}
	}
}
