package app

import (
	"context"
	"time"

	"github.com/codelieche/cronjob/apiserver/pkg/core"
	"github.com/codelieche/cronjob/apiserver/pkg/services"
	"github.com/codelieche/cronjob/apiserver/pkg/shard"
	"github.com/codelieche/cronjob/apiserver/pkg/store"
	"github.com/codelieche/cronjob/apiserver/pkg/utils/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Scheduler 定时任务调度器
//
// 负责管理系统的后台定时任务，包括：
// - 每日统计数据聚合（凌晨01:00）
// - TaskLog分片表维护（凌晨02:00）
// - 可扩展支持其他定时任务
//
// 🔥 多副本安全：使用分布式锁防止并发执行
// 🔥 架构层次：Scheduler -> Service -> Store -> Database
type Scheduler struct {
	cron            *cron.Cron
	statsAggregator *services.StatsAggregator
	shardManager    *shard.ShardManager // 🔥 TaskLog分片管理器
	locker          core.Locker         // 🔥 分布式锁
	cronJobService  core.CronJobService // 🔥 CronJob服务（遵循分层架构）
}

// NewScheduler 创建定时任务调度器实例
func NewScheduler(db *gorm.DB, locker core.Locker, cronJobService core.CronJobService) *Scheduler {
	// 创建Cron实例（带秒级精度）
	c := cron.New(cron.WithSeconds())

	// 创建统计聚合器（遵循分层架构）
	statsAggregatorStore := store.NewStatsAggregatorStore(db)
	statsAggregator := services.NewStatsAggregator(statsAggregatorStore)

	// 🔥 创建TaskLog分片管理器
	shardConfig := &shard.ShardConfig{
		TablePrefix:    "task_logs",
		ShardBy:        "created_at",
		ShardUnit:      "month",
		AutoCreateNext: true,
		CheckInterval:  "24h", // 保留配置字段（实际由cron控制）
	}
	shardManager := shard.NewShardManager(db, shardConfig)

	return &Scheduler{
		cron:            c,
		statsAggregator: statsAggregator,
		shardManager:    shardManager,
		locker:          locker,
		cronJobService:  cronJobService, // 🔥 注入Service层
	}
}

// Start 启动定时任务调度器
func (s *Scheduler) Start() error {
	logger.Info("启动定时任务调度器")

	// 🔥 任务1：每日统计数据聚合
	// Cron表达式：0 0 1 * * *  （每天凌晨1点执行）
	// 使用分布式锁，防止多副本并发执行
	_, err := s.cron.AddFunc("0 0 1 * * *", func() {
		ctx := context.Background()
		lockKey := "stats:aggregator:daily"

		// 🔥 尝试获取锁（10分钟过期，预留充足时间）
		lock, err := s.locker.TryAcquire(ctx, lockKey, 10*time.Minute)
		if err != nil {
			if err == core.ErrLockAlreadyAcquired {
				logger.Warn("统计数据聚合任务已被其他实例执行，跳过", zap.String("lock_key", lockKey))
				return
			}
			logger.Error("获取聚合任务锁失败", zap.String("lock_key", lockKey), zap.Error(err))
			return
		}
		defer lock.Release(ctx)

		logger.Info("开始执行每日统计数据聚合任务", zap.String("instance", "acquired_lock"))
		startTime := time.Now()

		// 聚合昨天的数据
		if err := s.statsAggregator.AggregateDailyStats(""); err != nil {
			logger.Error("每日统计数据聚合失败", zap.Error(err))
		} else {
			duration := time.Since(startTime)
			logger.Info("每日统计数据聚合成功",
				zap.Duration("duration", duration),
				zap.String("lock_key", lockKey))
		}
	})

	if err != nil {
		logger.Error("注册每日统计数据聚合任务失败", zap.Error(err))
		return err
	}

	// 🔥 任务2：初始化CronJob的NULL last_plan
	// Cron表达式：0 */10 * * * *（每10分钟执行一次）
	// 用于初始化新建CronJob的last_plan字段，避免无法调度
	// 使用分布式锁，防止多副本并发执行
	_, err = s.cron.AddFunc("0 */10 * * * *", func() {
		ctx := context.Background()
		lockKey := "cronjob:init:last_plan"

		// 🔥 尝试获取锁（5分钟过期）
		lock, err := s.locker.TryAcquire(ctx, lockKey, 5*time.Minute)
		if err != nil {
			if err == core.ErrLockAlreadyAcquired {
				logger.Warn("CronJob初始化任务已被其他实例执行，跳过", zap.String("lock_key", lockKey))
				return
			}
			logger.Error("获取CronJob初始化锁失败", zap.String("lock_key", lockKey), zap.Error(err))
			return
		}
		defer lock.Release(ctx)

		logger.Info("开始初始化CronJob的last_plan", zap.String("instance", "acquired_lock"))
		startTime := time.Now()

		// 🔥 调用Service层执行初始化（遵循分层架构）
		affectedRows, err := s.cronJobService.InitializeNullLastPlan(ctx)
		if err != nil {
			logger.Error("初始化CronJob的last_plan失败", zap.Error(err))
		} else {
			duration := time.Since(startTime)
			if affectedRows > 0 {
				logger.Info("CronJob的last_plan初始化成功",
					zap.Int64("affected_rows", affectedRows),
					zap.Duration("duration", duration),
					zap.String("lock_key", lockKey))
			}
		}
	})

	if err != nil {
		logger.Error("注册CronJob初始化任务失败", zap.Error(err))
		return err
	}

	// 🔥 任务3：TaskLog分片表维护
	// Cron表达式：0 0 2 * * *（每天凌晨2点执行，避免与统计聚合冲突）
	// 使用分布式锁，防止多副本并发执行
	_, err = s.cron.AddFunc("0 0 2 * * *", func() {
		ctx := context.Background()
		lockKey := "shard:tasklog:maintenance"

		// 🔥 尝试获取锁（10分钟过期）
		lock, err := s.locker.TryAcquire(ctx, lockKey, 10*time.Minute)
		if err != nil {
			if err == core.ErrLockAlreadyAcquired {
				logger.Warn("TaskLog分片维护任务已被其他实例执行，跳过", zap.String("lock_key", lockKey))
				return
			}
			logger.Error("获取TaskLog分片维护锁失败", zap.String("lock_key", lockKey), zap.Error(err))
			return
		}
		defer lock.Release(ctx)

		logger.Info("开始执行TaskLog分片维护任务", zap.String("instance", "acquired_lock"))
		startTime := time.Now()

		// 执行分片维护
		if err := s.shardManager.DailyMaintenance(); err != nil {
			logger.Error("TaskLog分片维护失败", zap.Error(err))
		} else {
			duration := time.Since(startTime)
			logger.Info("TaskLog分片维护成功",
				zap.Duration("duration", duration),
				zap.String("lock_key", lockKey))
		}
	})

	if err != nil {
		logger.Error("注册TaskLog分片维护任务失败", zap.Error(err))
		return err
	}

	// 🔥 立即执行一次CronJob的last_plan初始化（确保启动时所有激活的cronjob都能被调度）
	go func() {
		logger.Info("启动时立即执行CronJob的last_plan初始化")
		ctx := context.Background()

		// 🔥 调用Service层执行初始化（遵循分层架构）
		affectedRows, err := s.cronJobService.InitializeNullLastPlan(ctx)
		if err != nil {
			logger.Error("初始CronJob的last_plan初始化失败", zap.Error(err))
		} else if affectedRows > 0 {
			logger.Info("初始CronJob的last_plan初始化成功",
				zap.Int64("affected_rows", affectedRows))
		}
	}()

	// 🔥 立即执行一次TaskLog分片维护（确保当前月和下月表存在）
	go func() {
		logger.Info("启动时立即执行TaskLog分片维护")
		if err := s.shardManager.DailyMaintenance(); err != nil {
			logger.Error("初始TaskLog分片维护失败", zap.Error(err))
		} else {
			logger.Info("初始TaskLog分片维护成功")
		}
	}()

	// 启动调度器
	s.cron.Start()
	logger.Info("定时任务调度器已启动",
		zap.String("任务1", "统计聚合(凌晨01:00)"),
		zap.String("任务2", "CronJob初始化(每10分钟)"),
		zap.String("任务3", "TaskLog分片维护(凌晨02:00)"))

	return nil
}

// Stop 停止定时任务调度器
//
// 执行优雅关闭，等待所有正在运行的任务完成
// 🔥 注意：会阻塞直到所有任务执行完毕
func (s *Scheduler) Stop() {
	logger.Info("正在停止定时任务调度器")

	// 停止cron（会等待正在运行的任务完成）
	ctx := s.cron.Stop()
	<-ctx.Done()

	logger.Info("定时任务调度器已停止（所有任务已执行完毕）")
}

// GetStatsAggregator 获取统计聚合器实例（用于手动触发）
func (s *Scheduler) GetStatsAggregator() *services.StatsAggregator {
	return s.statsAggregator
}
