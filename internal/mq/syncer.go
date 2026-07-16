package mq

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type TargetSyncerConfig struct {
	// FetchTargetID 从redis里获取批量待更新的targeID
	FetchTargetIDFunc func(ctx context.Context, rds *redis.Client) ([]string, error)

	// CountKeyFunc 构造获取redis计数器的key
	CountKeyFunc func(id string) string

	// CountNumber 获取redis计数器的值
	CountNumberFunc func(ctx context.Context, rds *redis.Client, key []string) ([]string, error)

	// UpdateFunc 批量更新数据库里的数据
	UpdateFunc func(ctx context.Context, db *sql.DB, parames map[string]string) error

	// DeleteFunc 删除dirty:XXX的 SET 部分
	DeleteFunc func(ctx context.Context, rds *redis.Client, ids []string) error
}

type TargetSyncer struct {
	cfg          TargetSyncerConfig
	db           *sql.DB
	rds          *redis.Client
	wg           sync.WaitGroup
	stopOnce     sync.Once
	parentCtx    context.Context
	parentCancel context.CancelFunc
}

func NewTargetSyncer(cfg TargetSyncerConfig, db *sql.DB, rds *redis.Client) *TargetSyncer {
	ctx, cancel := context.WithCancel(context.Background())
	c := &TargetSyncer{
		cfg:          cfg,
		db:           db,
		rds:          rds,
		parentCtx:    ctx,
		parentCancel: cancel,
	}
	c.wg.Add(1)
	go c.run()
	return c
}

func (ts *TargetSyncer) run() {
	timer := time.NewTicker(1 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ts.parentCtx.Done():
			return
		case <-timer.C:
			ts.sync()
		}
	}
}

func (ts *TargetSyncer) sync() {
	async, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ids, err := ts.cfg.FetchTargetIDFunc(async, ts.rds)
	if err != nil {
		if err == redis.Nil {
			return
		}
		zap.L().Error("获取targetID失败", zap.Error(err))
		return
	}
	// 没有dirty:XXX
	if len(ids) == 0 {
		return
	}

	keys := make([]string, 0, len(ids))

	for _, id := range ids {
		key := ts.cfg.CountKeyFunc(id)
		keys = append(keys, key)
	}

	counts, err := ts.cfg.CountNumberFunc(async, ts.rds, keys)
	if err != nil {
		zap.L().Error("获取计数器失败", zap.Error(err))
		return
	}

	parames := make(map[string]string, len(ids))

	for idx, id := range ids {
		parames[id] = counts[idx]
	}

	err = ts.cfg.UpdateFunc(async, ts.db, parames)

	if err != nil {
		zap.L().Error(err.Error())
	} else {
		err = ts.cfg.DeleteFunc(async, ts.rds, ids)
		if err != nil {
			zap.L().Error(err.Error())
		}
	}
}

func (ts *TargetSyncer) Stop() {
	ts.stopOnce.Do(func() {
		ts.parentCancel()

		done := make(chan struct{})

		go func() {
			ts.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-time.After(5 * time.Second):
			zap.L().Info("定时更新赞数/关注数异步任务超时关闭")
		}
	})
}
