package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/levigross/grequests"
)

// 获取AccessToken 一切操作的前提
// 文档：https://ding-doc.dingtalk.com/doc#/serverapi2/eev437
// 请求方法：GET
// 请求地址：https://oapi.dingtalk.com/gettoken?appkey=key&appsecret=secret
// GET参数说明：
// appkey：必须，应用的唯一表示Key
// appsecret：必须，应用的秘钥
func (ding *DingDing) GetAccessToken() (accessToken string, err error) {
	// 定义变量
	var (
		url         string
		options     *grequests.RequestOptions
		response    *grequests.Response
		apiResponse ApiResponse
	)

	// 判断是否有Access Token
	if ding.AccessToken != "" {
		return ding.AccessToken, nil
	}

	// 通过API获取AccessToken
	url = fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s", ding.AppKey, ding.AppSecret)
	options = &grequests.RequestOptions{
		Headers:   map[string]string{"Content-Type": "application/json"},
		UserAgent: "",
		Host:      "",
	}

	if response, err = grequests.Get(url, options); err != nil {
		log.Println("获取AccessToken出错：", err.Error())
		return
	}

	apiResponse = ApiResponse{}

	if err = json.Unmarshal(response.Bytes(), &apiResponse); err != nil {
		return
	} else {
		if apiResponse.Errcode == 0 {
			accessToken = apiResponse.AccessToken
			return
		} else {
			msg := fmt.Sprintf("获取AccessToken出错, 错误代码是:%d,错误消息是:%s",
				apiResponse.Errcode, apiResponse.Errmsg)
			err = errors.New(msg)
			return
		}
	}

	// 对结果进行解析：通过gjson库
	//results := gjson.GetManyBytes(response.Bytes(), "errcode", "access_token", "errmsg")
	//for _, result := range results {
	//	log.Println(result)
	//	log.Println(result.Exists(), result.String(), result.Index, result.Type)
	//}
	//log.Println(results)

	return
}
