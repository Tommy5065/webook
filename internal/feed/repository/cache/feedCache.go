package cache

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type FeedCache struct {
	rds *redis.Client
}

func NewFeedCache(rds *redis.Client) *FeedCache {
	return &FeedCache{
		rds: rds,
	}
}

var offset int64
var lastID string

func (fc *FeedCache) GetFollowList(ctx context.Context, usrID int32, limit int) (articleIDList []int64, retError error) {
	key := "fanInBox:" + strconv.FormatInt(int64(usrID), 10)
	var Max string
	if lastID == "" {
		Max = "+inf"
	} else {
		Max = lastID
	}
	rdsZ, rdsError := fc.rds.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{Max: Max, Min: "-inf", Offset: offset, Count: int64(limit)}).Result()
	if rdsError != nil {
		zap.L().Error("拉取收件箱内容失败:", zap.Int32("用户id:", usrID), zap.Error(rdsError))
		return articleIDList, rdsError
	}

	if len(rdsZ) == 0 {
		return articleIDList, retError
	}

	for _, values := range rdsZ {
		valueString, _ := values.Member.(string)
		value, _ := strconv.ParseInt(valueString, 10, 64)
		articleIDList = append(articleIDList, value)
	}
	offset += int64(limit)
	lastID = strconv.FormatInt(articleIDList[len(articleIDList)-1], 10)
	return articleIDList, nil
}
