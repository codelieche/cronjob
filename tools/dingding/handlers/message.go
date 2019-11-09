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
		success       bool
		err           error
	)

	dingApp = dingding.NewDing()

	// 获取到用户以及获取消息内容
	userName := ctx.PostValue("user")
	mobile := ctx.PostValue("mobile")
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

	msg := &dingding.Message{
		MsgType: "text",
		Text:    &dingding.TextMsg{Content: content},
	}

	workerMessage = &dingding.WorkerMessage{
		AgentID:    dingApp.AgentId,
		UseridList: user.DingID,
		Msg:        msg,
	}

	if success, err = dingApp.SendWorkerMessage(workerMessage); err != nil {
		ctx.WriteString(err.Error())
	} else {
		ctx.JSON(iris.Map{"status": success, "message": "消息发送成功"})
	}
}
