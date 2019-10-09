package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/codelieche/cronjob/backend/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 日志处理的接口
type LogHandler interface {
	ConsumeLogsLoop()                              // 消费日志循环函数
	AddLog(executeLog *common.JobExecuteLog) error // 添加日志
	Stop()                                         // 日志处理器停止时的操作
}

// 日志处理器--mongo
type MongoLogHandler struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	logChan    chan *common.JobExecuteLog // 日志channel
	logList    []interface{}              // 日志列表
	Duration   int                        // 刷新日志的间隔(毫秒)
	isActive   bool                       // 是否启动
	writeLock  *sync.RWMutex              // 读写锁
}

// 把日志列表插入到数据库中
// 在ConsumeLogsLoop中和Stop中都会调用到
// 同时这个写日志不可并行操作
func (logHandler *MongoLogHandler) insertManayLogList() (insertManyResult *mongo.InsertManyResult, err error) {
	// 进入锁
	logHandler.writeLock.Lock()
	// 记得用defer释放锁
	defer logHandler.writeLock.Unlock()

	if len(logHandler.logList) > 0 {
		if insertManyResult, err = logHandler.collection.InsertMany(context.TODO(), logHandler.logList); err != nil {
			return nil, err
		} else {
			// 设置新的logList为空
			logHandler.logList = nil
			return insertManyResult, nil
		}
	} else {
		return nil, nil
	}

}

// 消费日志循环
func (logHandler *MongoLogHandler) ConsumeLogsLoop() {
	var (
		executeLog *common.JobExecuteLog
		//insertOneResult *mongo.InsertOneResult
		//logSlice []common.JobExecuteLog
		insertManyResult *mongo.InsertManyResult
		err              error
		logTimerDuration time.Duration
		timer            *time.Timer
	)
	// 状态设置为激活
	logHandler.isActive = true

	// 判断是否有读写锁
	if logHandler.writeLock == nil {
		logHandler.writeLock = &sync.RWMutex{}
	}

	// 如果初始化的时候没设置timer，就这里设置个
	if logHandler.Duration <= 0 {
		logTimerDuration = 1000 * time.Microsecond
	} else {
		logTimerDuration = time.Duration(logHandler.Duration) * time.Microsecond
	}
	// 创建一个timer
	timer = time.NewTimer(logTimerDuration)

	for logHandler.isActive {
		select {
		case executeLog = <-logHandler.logChan:
			if executeLog == nil {
				goto END
			}

			// log.Println("现在日志长度：", len(logHandler.logList))
			if len(logHandler.logList) >= 50 {
				// 需要写入一次数据了
				goto INSERT
			} else {

				// 加个锁：因为写入和置空可能有冲突
				logHandler.writeLock.Lock()
				logHandler.logList = append(logHandler.logList, *executeLog)
				logHandler.writeLock.Unlock() // 注意这里就别用defer

				// log.Println("现在日志长度：", len(logHandler.logList))
				// 这里需要填写后continue，要不会继续执行后面的INSERT语句
				continue
			}
		case <-timer.C:
			//log.Println("计时器，计时器")
			// 到了该刷新下日志的时候了
			// 记得执行：timer.Reset
			goto INSERT
			//log.Println(executeLog)

			// 插入对象到mongodb中
			//if insertOneResult, err = logHandler.collection.InsertOne(context.Background(), executeLog); err != nil {
			//	log.Println(err.Error())
			//	continue
			//} else {
			//	log.Println("插入数据成功：", insertOneResult.InsertedID)
			//}
		}
	INSERT:
		// 判断下日志是否有
		if len(logHandler.logList) > 0 {
			// 一次插入多条记录
			log.Println("开始写入大量数据：现在日志长度为", len(logHandler.logList))
			if insertManyResult, err = logHandler.insertManayLogList(); err != nil {
				log.Println("写入日志出错：", err.Error())
			} else {
				for _, item := range insertManyResult.InsertedIDs {
					objectID := item.(primitive.ObjectID)
					log.Println(objectID.Hex())
				}

				// 设置新的logList为空
				logHandler.logList = nil

			}
		} else {
			// 日志slice中没有数据，本次无需操作
			timer.Reset(logTimerDuration)
		}
		// 重置一下timer: 一定记得重置一下
		timer.Reset(logTimerDuration)
		// 继续for循环
	}

END:
	log.Println("跳出logHandler.ConsumeLogsLoop()")
	// 当日志处理器中还有日志未写入，那么我们就再次写入一下
	if len(logHandler.logList) > 0 {
		logHandler.insertManayLogList()
	}

	timer.Stop()
	return
}

// 保存日志操作
func (logHandler *MongoLogHandler) AddLog(executeLog *common.JobExecuteLog) (err error) {

	// 把新的日志加入到channel中
	logHandler.logChan <- executeLog
	return
}

func NewMongoLogHandler() (logHandler *MongoLogHandler, err error) {
	var (
		option         *options.ClientOptions
		database       *mongo.Database
		collection     *mongo.Collection
		client         *mongo.Client
		connectTimeout time.Duration
	)

	//	配置文件
	connectTimeout = 10 * time.Second
	option = &options.ClientOptions{
		Hosts: []string{"127.0.0.1:27017"},
		Auth: &options.Credential{
			Username: "root",
			Password: "password",
		},
		ConnectTimeout: &connectTimeout,
	}

	if client, err = mongo.Connect(context.TODO(), option); err != nil {
		return
	}

	// 选择数据库
	database = client.Database("cronjob")

	//log.Println(client)
	//log.Println(database.Name())

	//	插入单条记录
	//  选择表
	collection = database.Collection("logs")

	logHandler = &MongoLogHandler{
		client:     client,
		database:   database,
		collection: collection,
		logChan:    make(chan *common.JobExecuteLog, 1000),
		Duration:   5000, // 5000毫秒写一次日志
	}
	return
}

// 日志处理器停止时候的操作
// 停止的时候，需要把日志全部写入
// 当worker需要停止的时候，需要调度这些
func (logHandler *MongoLogHandler) Stop() {
	// 设置为isActive为false
	logHandler.isActive = false
	// 写入日志: 结果暂时不做处理
	logHandler.insertManayLogList()

	log.Println("记录日志处理器stop")
}
