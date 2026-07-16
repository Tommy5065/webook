package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ArticleCache struct {
	rds *redis.Client
}

func NewArticalCache(rds *redis.Client) *ArticleCache {
	return &ArticleCache{
		rds: rds,
	}
}

func (ac *ArticleCache) InBox(ctx context.Context, ids []int32, articleID, timeStamp int64) (retError error) {
	for _, id := range ids {
		key := fmt.Sprintf("fanInBox:%v", id)
		tx, finalErr := ac.rds.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.ZAdd(ctx, key, redis.Z{Member: articleID, Score: float64(timeStamp)})
			p.Expire(ctx, key, 15*24*time.Hour)
			return nil
		})

		if finalErr != nil {
			zap.L().Error("执行redis EXEC失败:", zap.String("业务类型:", "插入粉丝收件箱"), zap.String("收件箱编号:", key), zap.Error(finalErr))
			return finalErr
		}

		for _, i := range tx {
			if i.Err() != nil {
				zap.L().Error("redis事务内部命令失败:", zap.String("收件箱编号:", key), zap.String("命令名称:", i.Name()), zap.Error(i.Err()))
			}
		}
	}
	return retError
}
