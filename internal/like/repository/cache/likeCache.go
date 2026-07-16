package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type LikeCache struct {
	cmd *redis.Client
}

func NewLikeCache(rds *redis.Client) *LikeCache {
	return &LikeCache{
		cmd: rds,
	}
}

func (lc *LikeCache) SIsMember(ctx context.Context, key []string, args []interface{}) (bool, error) {
	return lc.cmd.SIsMember(ctx, key[0], args[0]).Result()
}

func (lc *LikeCache) IncrOrDesc(ctx context.Context, key []string, args []interface{}, parten string) (int64, error) {
	var tx []redis.Cmder
	var finalErr error

	switch parten {
	case "like":
		tx, finalErr = lc.cmd.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.SAdd(ctx, key[0], args[0])
			p.Expire(ctx, key[0], 24*time.Hour)
			p.Incr(ctx, key[1])
			p.Expire(ctx, key[1], 7*24*time.Hour)
			p.SAdd(ctx, key[2], args[1])
			return nil
		})
	case "disLike":
		tx, finalErr = lc.cmd.TxPipelined(ctx, func(p redis.Pipeliner) error {
			// 未执行事务前
			p.SRem(ctx, key[0], args[0])
			p.Decr(ctx, key[1])
			p.Expire(ctx, key[1], 7*24*time.Hour)
			p.SAdd(ctx, key[2], args[1])
			return nil
		})
	}
	// 1.先看执行EXEC是否成功
	if finalErr != nil {
		zap.L().Error("执行redis EXEC失败:", zap.String("业务类型:", "like"), zap.String("操作:", parten), zap.Error(finalErr))
		return 0, finalErr
	}

	// 2.看命令内部实现是否发送错误
	for _, i := range tx {
		if i.Err() != nil {
			zap.L().Error("内部命令执行失败:", zap.String("业务类型:", "like"), zap.String("操作:", parten), zap.String("命令名称", i.Name()), zap.Error(finalErr))
			return 0, i.Err()
		}
	}
	return lc.GetNumber(ctx, key)
}

func (lc *LikeCache) GetNumber(ctx context.Context, key []string) (int64, error) {
	number, err := lc.cmd.Get(ctx, key[1]).Int64()
	if err != nil {
		if err == redis.Nil {
			return number, redis.Nil
		}
		zap.L().Error("获取like计数器失败", zap.Error(err))
		return number, err
	}
	return number, err
}
