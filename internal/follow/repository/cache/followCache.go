package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type FollowCache struct {
	cmd *redis.Client
}

func NewFollowCache(rds *redis.Client) *FollowCache {
	return &FollowCache{
		cmd: rds,
	}
}

func (lf *FollowCache) SIsMember(ctx context.Context, key []string, args []interface{}) (bool, error) {
	return lf.cmd.SIsMember(ctx, key[1], args[0]).Result()
}

// 两个set--1.followed(key): follow:followed:fanID
// 2.following(key): follow:following:followingID
// 两个计数器 -- 1.follow:following:count -- 粉丝数
// 2. follow:followed:count -- 关注数
// 两个脏页-- 1.dirty:following B,A,C -- 该用户粉丝数有更新
// 2. dirty:followed E,D,F -- 该用户关注数有更新

func (lf *FollowCache) IncrORDesc(ctx context.Context, key []string, args []interface{}, parten string) (int64, error) {
	var tx []redis.Cmder
	var finalErr error

	switch parten {
	case "follow":
		tx, finalErr = lf.cmd.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.SAdd(ctx, key[0], args[1])
			p.Expire(ctx, key[0], 24*time.Hour)
			p.SAdd(ctx, key[1], args[0])
			p.Expire(ctx, key[1], 24*time.Hour)
			p.Incr(ctx, key[2])
			p.Expire(ctx, key[2], 7*24*time.Hour)
			p.Incr(ctx, key[3])
			p.Expire(ctx, key[3], 7*24*time.Hour)
			p.SAdd(ctx, key[4], args[1])
			p.SAdd(ctx, key[5], args[0])
			return nil
		})
	case "disFollow":
		tx, finalErr = lf.cmd.TxPipelined(ctx, func(p redis.Pipeliner) error {
			// 未执行事务前
			p.SRem(ctx, key[0], args[1])
			p.Expire(ctx, key[0], 24*time.Hour)
			p.SRem(ctx, key[1], args[0])
			p.Expire(ctx, key[1], 24*time.Hour)
			p.Decr(ctx, key[2])
			p.Expire(ctx, key[2], 7*24*time.Hour)
			p.Decr(ctx, key[3])
			p.Expire(ctx, key[3], 7*24*time.Hour)
			p.SAdd(ctx, key[4], args[1])
			p.SAdd(ctx, key[5], args[0])
			return nil
		})
	}

	// 1.先看执行EXEC是否成功
	if finalErr != nil {
		zap.L().Error("follow执行redis EXEC失败:", zap.String("业务类型:", "follow"), zap.String("操作:", parten), zap.Error(finalErr))
		return 0, finalErr
	}

	// 2.看命令内部实现是否发送错误
	for _, i := range tx {
		if i.Err() != nil {
			zap.L().Error("内部命令执行失败:", zap.String("业务类型:", "follow"), zap.String("操作:", parten), zap.String("命令名称", i.Name()), zap.Error(finalErr))
			return 0, i.Err()
		}
	}
	return lf.GetFanNumber(ctx, key)
}

func (lf *FollowCache) GetFanNumber(ctx context.Context, key []string) (int64, error) {
	number, err := lf.cmd.Get(ctx, key[3]).Int64()
	if err != nil {
		zap.L().Error("获取fan_number失败", zap.Error(err))
		return number, err
	}
	return number, err
}

func (lf *FollowCache) GetFollowingNumber(ctx context.Context, key []string) (int64, error) {
	number, err := lf.cmd.Get(ctx, key[2]).Int64()
	if err != nil {
		zap.L().Error("获取following_number失败", zap.Error(err))
		return number, err
	}
	return number, err
}
