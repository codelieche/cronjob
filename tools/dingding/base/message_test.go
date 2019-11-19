package base

import (
	"fmt"
	"log"
	"testing"
	"time"
)

// 发送工作消息测试
func TestDingDing_SendWorkerMessage(t *testing.T) {
	ding := NewDing()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	msg := &DingMessage{
		MsgType: "text",
		Text: &TextMsg{
			Content: fmt.Sprintf("这个是消息内容:%s", nowStr),
		},
	}

	msg2 := &DingMessage{
		MsgType: "markdown",
		Markdown: &MarkdownMsg{
			Title: "这个是标题内容",
			Text:  "> www.codelieche.com\n\n**你好，这个是测试内容**。\n" + nowStr,
		},
	}
	msg = msg2
	workMessage := WorkerMessage{
		AgentID:    ding.AgentId,
		UseridList: "manager5342",
		DeptIdList: "",
		ToAllUser:  false,
		Msg:        msg,
	}

	if success, err := ding.SendWorkerMessage(&workMessage, nil); err != nil {
		t.Error(err.Error())
		return
	} else {
		log.Println("发送消息结果为：", success)
	}
}
