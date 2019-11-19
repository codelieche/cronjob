package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/codelieche/cronjob/tools/dingding/base"

	"github.com/juju/errors"
	"github.com/kataras/iris"
)

// 发送工作消息给用户
func SendWorkerMessageToUser(ctx iris.Context) {
	// 定义变量
	var (
		dingApp         *base.DingDing
		workerMessage   *base.WorkerMessage
		user            *base.User
		userList        []*base.User // 消息接收用户列表
		useridListStr   string       // 接收消息用户的钉钉ip，多个以逗号分隔
		dingMessage     *base.DingMessage
		dingMessageData []byte
		message         *base.Message
		success         bool
		err             error
	)

	dingApp = base.NewDing()

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
		ctx.WriteString(err.Error())
		return
	} else {
		// 获取到用户
		if mobiles != "" {
			for _, mobile := range strings.Split(mobiles, ",") {
				// 根据手机号获取用户
				if user, err = base.GetUserByMobile(mobile); err != nil {
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
				if user, err = base.GetUserByName(userName); err != nil {
					panic(err)
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
		dingMessage = &base.DingMessage{
			MsgType: "text",
			Text:    &base.TextMsg{Content: content},
		}
	} else {
		if msgType == "markdown" {
			dingMessage = &base.DingMessage{
				MsgType: msgType,
				Markdown: &base.MarkdownMsg{
					Title: title,
					Text:  content,
				},
			}
		} else {
			err = errors.New("消息类型错误")
			ctx.WriteString(err.Error())
		}
	}

	workerMessage = &base.WorkerMessage{
		AgentID:    dingApp.AgentId,
		UseridList: useridListStr,
		Msg:        dingMessage,
	}

	if dingMessageData, err = json.Marshal(workerMessage); err != nil {
		log.Println(err.Error())
	}

	//	记录消息内容
	message = &base.Message{
		Success: false,
		//UserID:   user.ID,
		Users:    userList,
		Title:    title,
		MsgType:  msgType,
		Content:  content,
		DingData: dingMessageData,
	}

	if success, err = dingApp.SendWorkerMessage(workerMessage, message); err != nil {
		ctx.WriteString(err.Error())
	} else {
		ctx.JSON(iris.Map{"status": success, "message": "消息发送成功"})
	}
}

// 发送的消息列表api
func MessageListApi(ctx iris.Context) {
	// 定义变量
	var (
		page     int
		pageSize int
		offset   int
		limit    int
		messages []*base.Message
		err      error
	)

	//	得到page
	page = ctx.Params().GetIntDefault("page", 1)
	pageSize = ctx.URLParamIntDefault("pageSize", 10)

	limit = pageSize
	if page > 1 {
		offset = (page - 1) * pageSize
	}

	// 获取用户
	if messages, err = base.GetMessageList(offset, limit); err != nil {
		log.Println(err)
		ctx.HTML("<div>%s</div>", err.Error())
	} else {
		ctx.JSON(messages)
	}
}

// 消息详情
func GetMessageDetailApi(ctx iris.Context) {
	var (
		msgID   int
		message *base.Message
		err     error
	)

	if msgID, err = ctx.Params().GetInt("id"); err != nil {
		ctx.WriteString(err.Error())
		return
	}

	if message, err = base.GetMessageByid(msgID); err != nil {
		if err == base.NotFountError {
			ctx.WriteString(err.Error())
		}
	} else {
		ctx.JSON(message)
	}
}
