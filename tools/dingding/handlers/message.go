package handlers

import (
	"encoding/json"
	"log"

	"github.com/codelieche/cronjob/tools/dingding"
	"github.com/juju/errors"
	"github.com/kataras/iris"
)

// 发送工作消息给用户
func SendWorkerMessageToUser(ctx iris.Context) {
	// 定义变量
	var (
		dingApp         *dingding.DingDing
		workerMessage   *dingding.WorkerMessage
		user            *dingding.User
		dingMessage     *dingding.DingMessage
		dingMessageData []byte
		message         *dingding.Message
		success         bool
		err             error
	)

	dingApp = dingding.NewDing()

	// 获取到用户以及获取消息内容
	userName := ctx.PostValue("user")
	mobile := ctx.PostValue("mobile")
	msgType := ctx.PostValueDefault("type", "text")
	title := ctx.PostValueDefault("title", "通知消息")

	content := ctx.PostValue("content")
	if userName == "" && mobile == "" {
		err = errors.New("用户名/手机号不可为空")
		log.Println(err)
		panic(err)
	} else {
		// 获取到用户
		if mobile != "" {
			// 根据手机号获取用户
			if user, err = dingding.GetUserByMobile(mobile); err != nil {
				panic(err)
			}
		} else {
			if user, err = dingding.GetUserByName(userName); err != nil {
				panic(err)
				log.Println(err.Error())
			}
		}

	}

	if content == "" {
		err = errors.New("消息内容不可为空")
		log.Println(err)
		panic(err)
	}

	if msgType == "text" {
		dingMessage = &dingding.DingMessage{
			MsgType: "text",
			Text:    &dingding.TextMsg{Content: content},
		}
	} else {
		if msgType == "markdown" {
			dingMessage = &dingding.DingMessage{
				MsgType: msgType,
				Markdown: &dingding.MarkdownMsg{
					Title: title,
					Text:  content,
				},
			}
		} else {
			err = errors.New("消息类型错误")
			ctx.WriteString(err.Error())
		}
	}

	workerMessage = &dingding.WorkerMessage{
		AgentID:    dingApp.AgentId,
		UseridList: user.DingID,
		Msg:        dingMessage,
	}

	if dingMessageData, err = json.Marshal(workerMessage); err != nil {
		log.Println(err.Error())
	}

	//	记录消息内容
	message = &dingding.Message{
		Success:  false,
		UserID:   user.ID,
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
		messages []*dingding.Message
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
	if messages, err = dingding.GetMessageList(offset, limit); err != nil {
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
		message *dingding.Message
		err     error
	)

	if msgID, err = ctx.Params().GetInt("id"); err != nil {
		ctx.WriteString(err.Error())
		return
	}

	if message, err = dingding.GetMessageByid(msgID); err != nil {
		if err == dingding.NotFountError {
			ctx.WriteString(err.Error())
		}
	} else {
		ctx.JSON(message)
	}
}
