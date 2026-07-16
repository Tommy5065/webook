package service

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"webookApp/internal/like/domain"
	"webookApp/internal/like/repository/cache"
	"webookApp/internal/sse"

	"go.uber.org/zap"
)

const (
	key1 = "article:usr:"
	key2 = "article:count:"
	key3 = "dirty:article"
)

type LikeService struct {
	// mqRepo     *domain.LikeRepository
	likeCache  *cache.LikeCache
	sseManager *sse.SSEManagerHandler // 推送通知系统
	mqProducer *domain.LikeProducer   // 点赞系统消息队列生产者
}

func NewLikeService(likecache *cache.LikeCache, mqproducer *domain.LikeProducer, ssemanager *sse.SSEManagerHandler) *LikeService {
	c := &LikeService{
		likeCache:  likecache,
		mqProducer: mqproducer,
		sseManager: ssemanager,
	}
	return c
}

func (ls *LikeService) Like(ctx context.Context, passivity int64, driving, articleUserID int32, title, username string) (int64, error) {
	keys := make([]string, 0, 3)
	args := make([]interface{}, 0, 2)

	// 组合要使用到的key和args
	key1_1 := key1 + strconv.FormatInt(passivity, 10)
	key2_2 := key2 + strconv.FormatInt(passivity, 10)
	keys = append(keys, key1_1, key2_2, key3)
	args = append(args, driving, passivity)

	var newCount int64
	var finalError error

	// 先查当前有多少赞,为了刷赞复用
	currentCount, _ := ls.likeCache.GetNumber(ctx, keys)

	exists, err := ls.likeCache.SIsMember(ctx, keys, args)
	// 说明sismember命令有问题或者redis挂了
	if err != nil {
		return currentCount, err
	}

	if exists {
		// 已点赞
		zap.L().Warn("有用户挂机刷:", zap.Int32("用户ID:", driving))
		return currentCount, nil
	}
	newCount, finalError = ls.likeCache.IncrOrDesc(ctx, keys, args, "like")
	// 加赞失败返回旧值
	if finalError != nil {
		return currentCount, finalError
	}

	go func() { // 往消息队列发送消息--消息发生成功调用通知api
		// 使用context给协程明确退出路径
		asyncCxt, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := ls.sendMsg(asyncCxt, []interface{}{"biz", "article", "action", "like", "driving_id", strconv.FormatInt(int64(driving), 10), "passivity_id", strconv.FormatInt(passivity, 10), "timestamp", time.Now().UTC().UnixMilli()})
		if err != nil {
			zap.L().Error("like生产者发送消息错误:", zap.String("类型:", "点赞"), zap.Error(err))
			return
		}
		// 给别人作者发通知
		message := fmt.Sprintf("%s likes your article:%s", username, title)
		ls.sseManager.SentToUser(articleUserID, message)
	}()
	return newCount, finalError
}

func (ls *LikeService) DisLike(ctx context.Context, passivity int64, driving int32) (int64, error) {
	keys := make([]string, 0, 3)
	args := make([]interface{}, 0, 2)

	// 组合要使用到的key和args
	key1_1 := key1 + strconv.FormatInt(passivity, 10)
	key2_2 := key2 + strconv.FormatInt(passivity, 10)
	keys = append(keys, key1_1, key2_2, key3)
	args = append(args, driving, passivity)

	var newCount int64
	var finalError error

	// 先查当前有多少赞,为了复用
	currentCount, _ := ls.likeCache.GetNumber(ctx, keys)

	exists, err := ls.likeCache.SIsMember(ctx, keys, args)
	// 说明sismember命令有问题或者redis挂了
	if err != nil {
		return currentCount, err
	}

	if !exists {
		return currentCount, nil
	}

	// set有效且用户存在
	newCount, finalError = ls.likeCache.IncrOrDesc(ctx, keys, args, "disLike")
	// 减赞失败返回旧值
	if finalError != nil {
		return currentCount, finalError
	}

	go func() { // 往消息队列发送消息
		// 使用context给协程明确退出路径
		asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := ls.sendMsg(asyncCtx, []interface{}{"biz", "article", "action", "disLike", "driving_id", strconv.FormatInt(int64(driving), 10), "passivity_id", strconv.FormatInt(passivity, 10), "timestamp", time.Now().UTC().UnixMilli()})
		if err != nil {
			zap.L().Error("like生产者发送消息错误:", zap.String("类型:", "取消点赞"), zap.Error(err))
		}
	}()
	return newCount, finalError
}

func (ls *LikeService) sendMsg(ctx context.Context, values []interface{}) (string, error) {
	return ls.mqProducer.Producer.Add(ctx, values)
}
