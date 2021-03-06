package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/codelieche/cronjob/backend/common/datasources"

	"github.com/codelieche/cronjob/backend/common/datamodels"

	"go.mongodb.org/mongo-driver/bson"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/codelieche/cronjob/backend/common"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 日志处理的接口
type LogHandler interface {
	ConsumeLogsLoop()                                  // 消费日志循环函数
	AddLog(executeLog *datamodels.JobExecuteLog) error // 添加日志
	Stop()                                             // 日志处理器停止时的操作
	//List(page int, pageSize int) (logList []*datamodels.JobExecuteLog, err error) //获取日志的列表
}

// 日志处理器--mongo
type MongoLogHandler struct {
	mongo     *datasources.MongoDB
	logChan   chan *datamodels.JobExecuteLog // 日志channel
	logList   []interface{}                  // 日志列表
	Duration  int                            // 刷新日志的间隔(毫秒)
	isActive  bool                           // 是否启动
	writeLock *sync.RWMutex                  // 读写锁
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
		if insertManyResult, err = logHandler.mongo.Collection.InsertMany(context.TODO(), logHandler.logList); err != nil {
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
		executeLog *datamodels.JobExecuteLog
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
		logTimerDuration = 1000 * time.Millisecond
	} else {
		logTimerDuration = time.Duration(logHandler.Duration) * time.Millisecond
		log.Println("写入日志的频率为：", logTimerDuration)

	}
	// 创建一个timer
	timer = time.NewTimer(logTimerDuration)
	timer.Reset(logTimerDuration)

	for logHandler.isActive {
		//log.Println("for ------")
		select {
		case executeLog = <-logHandler.logChan:
			// log.Println(executeLog)
			if executeLog == nil {
				goto END
			}

			// log.Println("现在日志长度：", len(logHandler.logList))
			// 加个锁：因为写入和置空可能有冲突
			logHandler.writeLock.Lock()
			logHandler.logList = append(logHandler.logList, *executeLog)
			logHandler.writeLock.Unlock() // 注意这里就别用defer

			// 如果日志列表的长度大于50了，那需要执行一下插入
			if len(logHandler.logList) >= 50 {
				// 需要写入一次数据了
				goto INSERT
			} else {
				// log.Println("现在日志长度：", len(logHandler.logList))
				// 这里需要填写后continue，要不会继续执行后面的INSERT语句
				continue
			}
		case <-timer.C:
			// log.Println("计时器，计时器")
			// log.Println(logTimerDuration)

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
			// log.Println("开始写入大量数据：现在日志长度为", len(logHandler.logList))
			if insertManyResult, err = logHandler.insertManayLogList(); err != nil {
				log.Printf("写入日志(%d条)出错：%s\n", len(logHandler.logList), err.Error())
				// 出错了之后，延时一会
				time.Sleep(10 * time.Second)

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
func (logHandler *MongoLogHandler) AddLog(executeLog *datamodels.JobExecuteLog) (err error) {

	// 把新的日志加入到channel中
	logHandler.logChan <- executeLog
	return
}

// 获取日志的列表
func (logHandler *MongoLogHandler) List(page int, pageSize int) (logList []*datamodels.JobExecuteLog, err error) {
	var (
		skip  int64
		limit int64

		filter  *JobExecuteLogFilter
		logSort *SortLogByStartTime

		cursor        *mongo.Cursor
		jobExecuteLog *datamodels.JobExecuteLog
	)
	// 对page进行判断
	if page < 0 {
		err = errors.New("日志的列表page需要大于0")
		return
	}
	// 需要跳过多少
	if page > 1 {
		skip = int64((page - 1) * pageSize)
	}
	// 显示的条数
	limit = int64(pageSize)

	// 过滤条件
	filter = &JobExecuteLogFilter{Name: "test1"}
	filter = filter
	//log.Println(filter)

	logSort = &SortLogByStartTime{StartTime: -1}

	// log.Println(skip, limit)

	if cursor, err = logHandler.mongo.Collection.Find(
		context.TODO(), bson.D{},
		&options.FindOptions{Skip: &skip, Limit: &limit, Sort: &logSort},
	); err != nil {
		return
	}

	// 延迟释放游标
	defer cursor.Close(context.TODO())

	// 开始处理查找结果
	for cursor.Next(context.TODO()) {
		jobExecuteLog = &datamodels.JobExecuteLog{}
		if err = cursor.Decode(&jobExecuteLog); err != nil {
			log.Println(err)
			continue
		} else {
			logList = append(logList, jobExecuteLog)
		}
		//log.Println(jobExecuteLog)
	}

	return logList, nil
}

func NewMongoLogHandler(mongoConfig *common.MongoConfig) (logHandler *MongoLogHandler, err error) {

	//	获取mongodb的连接
	mongoDB := datasources.GetMongoDB()

	logHandler = &MongoLogHandler{
		mongo:    mongoDB,
		logChan:  make(chan *datamodels.JobExecuteLog, 1000),
		Duration: 5000, // 5000毫秒写一次日志
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
