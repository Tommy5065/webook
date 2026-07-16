package service

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"webookApp/internal/follow/domain"
	"webookApp/internal/follow/repository/cache"
	"webookApp/internal/sse"

	"go.uber.org/zap"
)

const (
	key1 = "follow:followed:"
	key2 = "follow:following:"
	key3 = "follow:following:count:" // 粉丝数
	key4 = "follow:followed:count:"  // 关注数
	key5 = "dirty:following"
	key6 = "dirty:followed"
)

type FollowService struct {
	followCache *cache.FollowCache
	sseManager  *sse.SSEManagerHandler // 推送消息系统
	mqProducer  *domain.FollowProducer // 消息队列生产者
}

func NewFollowService(followcache *cache.FollowCache, mqproducer *domain.FollowProducer, sse *sse.SSEManagerHandler) *FollowService {
	c := &FollowService{
		followCache: followcache,
		mqProducer:  mqproducer,
		sseManager:  sse,
	}
	return c
}

func (ls *FollowService) Follow(ctx context.Context, passivity int64, driving int32, username string) (int64, error) {
	keys := make([]string, 0, 6)
	args := make([]interface{}, 0, 2)

	// 组合要使用到的key和args
	key1_1 := key1 + strconv.FormatInt(int64(driving), 10)
	key2_2 := key2 + strconv.FormatInt(passivity, 10)
	key3_3 := key3 + strconv.FormatInt(passivity, 10)
	key4_4 := key4 + strconv.FormatInt(int64(driving), 10)
	keys = append(keys, key1_1, key2_2, key3_3, key4_4, key5, key6)
	args = append(args, driving, passivity)

	var newCount int64
	var finalError error

	// 先查当前有多少粉丝数,为了刷赞复用
	currentCount, _ := ls.followCache.GetFanNumber(ctx, keys)

	exists, err := ls.followCache.SIsMember(ctx, keys, args)
	// 说明sismember命令有问题或者redis挂了
	if err != nil {
		return currentCount, err
	}

	if exists {
		// 已点赞
		zap.L().Warn("有用户重复关注:", zap.Int32("用户ID:", driving))
		return currentCount, nil
	}
	newCount, finalError = ls.followCache.IncrORDesc(ctx, keys, args, "follow")
	// 加赞失败返回旧值
	if finalError != nil {
		return currentCount, finalError
	}

	go func() { // 往消息队列发送消息
		// 使用context给协程明确退出路径
		asyncCxt, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := ls.sendMsg(asyncCxt, []interface{}{"biz", "follow", "action", "follow", "driving_id", strconv.FormatInt(int64(driving), 10), "passivity_id", strconv.FormatInt(int64(passivity), 10), "timestamp", time.Now().UTC().UnixMilli()})
		if err != nil {
			zap.L().Error("follow生产者发送消息错误:", zap.String("操作:", "关注"), zap.Error(err))
			return
		}

		message := fmt.Sprintf("%s 关注了您", username)
		ls.sseManager.SentToUser(int32(passivity), message)

	}()
	return newCount, finalError

}

func (ls *FollowService) DisFollow(ctx context.Context, passivity int64, driving int32) (int64, error) {
	keys := make([]string, 0, 4)
	args := make([]interface{}, 0, 2)

	// 组合要使用到的key和args
	key1_1 := key1 + strconv.FormatInt(int64(driving), 10)
	key2_2 := key2 + strconv.FormatInt(passivity, 10)
	key3_3 := key3 + strconv.FormatInt(passivity, 10)
	key4_4 := key4 + strconv.FormatInt(int64(driving), 10)
	keys = append(keys, key1_1, key2_2, key3_3, key4_4, key5, key6)
	args = append(args, driving, passivity)

	var newCount int64
	var finalError error

	// 先查当前有多少赞,为了复用
	currentCount, _ := ls.followCache.GetFanNumber(ctx, keys)

	exists, err := ls.followCache.SIsMember(ctx, keys, args)
	// 说明sismember命令有问题或者redis挂了
	if err != nil {
		return currentCount, err
	}

	if !exists {
		return currentCount, nil
	}

	// set有效且用户存在
	newCount, finalError = ls.followCache.IncrORDesc(ctx, keys, args, "disFollow")
	// 减赞失败返回旧值
	if finalError != nil {
		return currentCount, finalError
	}

	go func() { // 往消息队列发送消息
		// 使用context给协程明确退出路径
		asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := ls.sendMsg(asyncCtx, []interface{}{"biz", "follow", "action", "disFollow", "driving_id", strconv.FormatInt(int64(driving), 10), "passivity_id", strconv.FormatInt(int64(passivity), 10), "timestamp", time.Now().UTC().UnixMilli()})
		if err != nil {
			zap.L().Error("follow生产者发送消息错误:", zap.String("操作:", "取消关注"), zap.Error(err))
		}
	}()
	return newCount, finalError
}

func (ls *FollowService) sendMsg(ctx context.Context, values []interface{}) (string, error) {
	return ls.mqProducer.Producer.Add(ctx, values)
}
