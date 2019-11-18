package repositories

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/codelieche/cronjob/tools/dingding/common"
	"github.com/codelieche/cronjob/tools/dingding/datasource"
)

// 通过ID获取消息
func TestMessageRepository_GetById(t *testing.T) {
	var msgId int64 = 1

	db := datasource.DB
	ding := common.NewDing()
	r := NewMessageRepository(db, ding)

	if message, err := r.GetById(msgId); err != nil {
		t.Error(err.Error())
	} else {
		log.Println(message.ID, message.Title, message.Success, message.Users, len(message.Content))
	}
}

// 获取消息列表
func TestMessageRepository_List(t *testing.T) {
	var offset int = 0
	var limit int = 1
	var haveNext = true

	// 先实例化Repository
	db := datasource.DB
	ding := common.NewDing()
	r := NewMessageRepository(db, ding)

	for haveNext {
		if messages, err := r.List(offset, limit); err != nil {
			t.Error(err.Error())
		} else {
			// 判断是否有下一页
			if len(messages) != limit {
				haveNext = false
			} else {
				offset += limit
			}
			// 输出获取到的消息
			for i, message := range messages {
				log.Println(i, message.ID, message.Title, message.Users)
			}
		}
	}
}

// 发送工作消息
func TestMessageRepository_SendWorkerMessage(t *testing.T) {

	db := datasource.DB
	ding := common.NewDing()

	r := NewMessageRepository(db, ding)

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	msg := &common.DingMessage{
		MsgType: "text",
		Text: &common.TextMsg{
			Content: fmt.Sprintf("这个是消息内容:%s", nowStr),
		},
	}

	msg2 := &common.DingMessage{
		MsgType: "markdown",
		Markdown: &common.MarkdownMsg{
			Title: "这个是标题内容",
			Text:  "> www.codelieche.com\n\n**你好，这个是测试内容**。\n" + nowStr,
		},
	}
	msg = msg2
	workMessage := common.WorkerMessage{
		AgentID:    ding.AgentId,
		UseridList: "manager5342",
		DeptIdList: "",
		ToAllUser:  false,
		Msg:        msg,
	}

	if success, err := r.SendWorkerMessage(&workMessage, nil); err != nil {
		t.Error(err.Error())
	} else {
		log.Println("发送消息结果为：", success)
	}
}
