package shard

import (
	"sync"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"go.uber.org/zap"
)

// ShardScheduler 分片调度器
// 负责定期执行分片表的维护任务
type ShardScheduler struct {
	shardManager *ShardManager
	ticker       *time.Ticker
	stopChan     chan struct{}
	wg           sync.WaitGroup
	running      bool
	mutex        sync.RWMutex
}

// NewShardScheduler 创建分片调度器
func NewShardScheduler(shardManager *ShardManager) *ShardScheduler {
	return &ShardScheduler{
		shardManager: shardManager,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动分片调度器
func (s *ShardScheduler) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return nil
	}

	// 解析检查间隔
	interval, err := time.ParseDuration(s.shardManager.config.CheckInterval)
	if err != nil {
		logger.Error("解析分片检查间隔失败", zap.Error(err))
		interval = 24 * time.Hour // 默认24小时
	}

	s.ticker = time.NewTicker(interval)
	s.running = true

	// 启动维护任务协程
	s.wg.Add(1)
	go s.maintenanceLoop()

	// 立即执行一次维护任务
	go func() {
		if err := s.runMaintenance(); err != nil {
			logger.Error("初始分片维护任务失败", zap.Error(err))
		}
	}()

	logger.Info("分片调度器已启动",
		zap.Duration("interval", interval),
		zap.Bool("auto_create_next", s.shardManager.config.AutoCreateNext))

	return nil
}

// Stop 停止分片调度器
func (s *ShardScheduler) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)

	if s.ticker != nil {
		s.ticker.Stop()
	}

	// 等待维护任务完成
	s.wg.Wait()

	logger.Info("分片调度器已停止")
}

// IsRunning 检查调度器是否正在运行
func (s *ShardScheduler) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// RunMaintenanceNow 立即执行一次维护任务
func (s *ShardScheduler) RunMaintenanceNow() error {
	return s.runMaintenance()
}

// maintenanceLoop 维护任务循环
func (s *ShardScheduler) maintenanceLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ticker.C:
			if err := s.runMaintenance(); err != nil {
				logger.Error("定时分片维护任务失败", zap.Error(err))
			}
		case <-s.stopChan:
			logger.Info("分片维护任务循环已停止")
			return
		}
	}
}

// runMaintenance 执行维护任务
func (s *ShardScheduler) runMaintenance() error {
	logger.Info("开始执行分片维护任务")
	startTime := time.Now()

	// 执行分片管理器的日常维护
	if err := s.shardManager.DailyMaintenance(); err != nil {
		logger.Error("分片维护任务失败",
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return err
	}

	logger.Info("分片维护任务完成",
		zap.Duration("duration", time.Since(startTime)))

	return nil
}

// GetMaintenanceStatus 获取维护状态信息
func (s *ShardScheduler) GetMaintenanceStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	status := map[string]interface{}{
		"running":        s.running,
		"check_interval": s.shardManager.config.CheckInterval,
		"auto_create":    s.shardManager.config.AutoCreateNext,
		"table_prefix":   s.shardManager.config.TablePrefix,
	}

	if s.ticker != nil {
		// 这里可以添加更多状态信息，比如下次执行时间等
		status["next_run"] = "定时执行中"
	}

	return status
}
