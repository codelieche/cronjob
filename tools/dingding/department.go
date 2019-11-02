package dingding

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/levigross/grequests"
)

// 部门相关api
// 请求方式：GET（HTTPS）
// 请求地址：https://oapi.dingtalk.com/department/list?access_token=ACCESS_TOKEN
func (ding *DingDing) ListDepartment() (departments []*Department, err error) {
	var (
		url         string
		accessToken string
		ro          *grequests.RequestOptions
		response    *grequests.Response
		apiResponse *ApiResponse
	)
	if accessToken, err = ding.GetAccessToken(); err != nil {
		log.Println(err.Error())
		return nil, err
	}
	url = fmt.Sprintf("https://oapi.dingtalk.com/department/list?access_token=%s", accessToken)

	ro = &grequests.RequestOptions{
		Headers: map[string]string{"Content-Type": "application/json"},
	}
	if response, err = grequests.Get(url, ro); err != nil {
		log.Println(err.Error())
		return nil, err
	} else {
		// 对响应的结果进行解析
		apiResponse = &ApiResponse{}
		if err = json.Unmarshal(response.Bytes(), apiResponse); err != nil {
			log.Println(err.Error())
			return nil, err
		} else {
			// 判断是否获取结果成功
			if apiResponse.Errcode != 0 {
				msg := fmt.Sprintf("获取数据出错，错误代码:%d(%s)", apiResponse.Errcode, apiResponse.Errmsg)
				err = errors.New(msg)
				return nil, err
			} else {
				// 到这里获取的结果正确
				return apiResponse.Department, nil
			}
		}
	}
}
