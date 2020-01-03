package sockets

import (
	"context"
	"log"
	"time"

	"github.com/codelieche/cronjob/backend/common"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/codelieche/cronjob/backend/common/datasources"
)

// 推送jobs给客户端
func pushJobsToClient(client *Client) (err error) {
	// 定义变量
	var (
		getResponse *clientv3.GetResponse
		ctx         context.Context
		keyValue    *mvccpb.KeyValue
	)
	// 先通过etcd获取到所有的jobs信息
	etcd := datasources.GetEtcd()

	ctx, _ = context.WithTimeout(context.Background(), time.Second*10)
	if getResponse, err = etcd.KV.Get(
		ctx,
		common.ETCD_JOBS_DIR,
		clientv3.WithPrefix(),
	); err != nil {
		log.Println(err)
		return err
	}
	// 获取响应中的消息
	for _, keyValue = range getResponse.Kvs {
		// 需要把所有的keyValue发送给客户端
		if err = client.SendMessage(1, keyValue.Value, true); err != nil {
			log.Println("发送消息失败：", err)
			break
		} else {
			// 发送消息成功
		}
	}
	return
}
