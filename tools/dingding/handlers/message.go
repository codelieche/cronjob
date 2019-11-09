package handlers

import (
	"log"

	"cronjob.codelieche/tools/dingding"
	"github.com/juju/errors"
	"github.com/kataras/iris"
)

// 发送工作消息给用户
func SendWorkerMessageToUser(ctx iris.Context) {
	// 定义变量
	var (
		dingApp       *dingding.DingDing
		workerMessage *dingding.WorkerMessage
		user          *dingding.User
		dingMessage   *dingding.Message
		success       bool
		err           error
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
		dingMessage = &dingding.Message{
			MsgType: "text",
			Text:    &dingding.TextMsg{Content: content},
		}
	} else {
		if msgType == "markdown" {
			dingMessage = &dingding.Message{
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

	if success, err = dingApp.SendWorkerMessage(workerMessage); err != nil {
		ctx.WriteString(err.Error())
	} else {
		ctx.JSON(iris.Map{"status": success, "message": "消息发送成功"})
	}
}
