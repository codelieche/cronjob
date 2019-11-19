package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datamodels"
	"github.com/codelieche/cronjob/tools/dingding/web/services"
	"github.com/kataras/iris"
)

type MessageController struct {
	Service services.MessageService
}

func (c *MessageController) GetBy(ctx iris.Context, id int64) (message *datamodels.Message, success bool) {
	// 定义变量
	var (
		err error
	)
	// 通过id获取消息详情
	if message, err = c.Service.GetById(id); err != nil {
		return nil, false
	} else {
		return message, true
	}
}

// 获取消息列表
func (c *MessageController) GetList(ctx iris.Context) (messages []*datamodels.Message, success bool) {
	return c.GetListBy(ctx, 1)
}

// 获取消息列表
func (c *MessageController) GetListBy(ctx iris.Context, page int) (messages []*datamodels.Message, success bool) {
	// 定义变量
	var (
		offset   int
		limit    int
		pageSize int
		err      error
	)

	pageSize = ctx.URLParamIntDefault("pageSize", 10)
	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	//	获取消息列表
	if messages, err = c.Service.GetList(offset, limit); err != nil {
		return nil, false
	} else {
		return messages, true
	}
}

// 发送工作消息
func (c *MessageController) PostCreate(ctx iris.Context) (message *datamodels.Message, err error) {
	// 定义变量
	var (
		workerMessage   *common.WorkerMessage
		user            *datamodels.User
		userList        []*datamodels.User // 消息接收用户列表
		useridListStr   string             // 接收消息用户的钉钉ip，多个以逗号分隔
		dingMessage     *common.DingMessage
		dingMessageData []byte
	)

	// 获取到用户以及获取消息内容
	userNames := ctx.PostValue("users") // 一次是可以给多个用户发送消息的,分割
	mobiles := ctx.PostValue("mobiles")
	msgType := ctx.PostValueDefault("type", "text")
	title := ctx.PostValueDefault("title", "通知消息")

	content := ctx.PostValue("content")
	if userNames == "" && mobiles == "" {
		err = errors.New("用户名/手机号不可为空")
		log.Println(err)
		ctx.StatusCode(400)
		//ctx.WriteString(err.Error())
		return nil, err
	} else {
		// 获取到用户
		if mobiles != "" {
			for _, mobile := range strings.Split(mobiles, ",") {
				// 根据手机号获取用户
				if user, err = c.Service.GetUserByMobile(mobile); err != nil {
					panic(err)
				} else {
					// 把用户加入到列表中
					userList = append(userList, user)

					if useridListStr == "" {
						useridListStr = user.DingID
					} else {
						useridListStr += fmt.Sprintf(",%s", user.DingID)
					}
				}
			}

		} else {
			for _, userName := range strings.Split(userNames, ",") {
				if user, err = c.Service.GetUserByName(userName); err != nil {
					//panic(err)
					log.Println(err.Error())
				} else {
					// 把用户加入到列表中
					userList = append(userList, user)
					if useridListStr == "" {
						useridListStr = user.DingID
					} else {
						useridListStr += fmt.Sprintf(",%s", user.DingID)
					}
				}
			}
		}
	}

	if content == "" {
		err = errors.New("消息内容不可为空")
		log.Println(err)
		//panic(err)
		ctx.StatusCode(400)
		ctx.WriteString(err.Error())
		return
	}

	// 判断用户是否为空
	if len(userList) < 1 {
		err = errors.New("传入的用户为空")
		log.Println(err.Error())
		ctx.StatusCode(400)
		ctx.WriteString(err.Error())
		return
	}

	if msgType == "text" {
		dingMessage = &common.DingMessage{
			MsgType: "text",
			Text:    &common.TextMsg{Content: content},
		}
	} else {
		if msgType == "markdown" {
			dingMessage = &common.DingMessage{
				MsgType: msgType,
				Markdown: &common.MarkdownMsg{
					Title: title,
					Text:  content,
				},
			}
		} else {
			err = errors.New("消息类型错误")
			ctx.WriteString(err.Error())
		}
	}

	workerMessage = &common.WorkerMessage{
		//AgentID:    dingApp.AgentId,
		UseridList: useridListStr,
		Msg:        dingMessage,
	}

	if dingMessageData, err = json.Marshal(workerMessage); err != nil {
		log.Println(err.Error())
	}

	//	记录消息内容
	message = &datamodels.Message{
		Success: false,
		//UserID:   user.ID,
		Users:    userList,
		Title:    title,
		MsgType:  msgType,
		Content:  content,
		DingData: dingMessageData,
	}

	if message, err = c.Service.SendWorkMessage(workerMessage, message); err != nil {
		//ctx.WriteString(err.Error())
		return nil, err
	} else {
		//ctx.JSON(iris.Map{"status": success, "message": "消息发送成功"})
		return message, nil
	}
}
