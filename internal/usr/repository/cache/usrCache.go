package cache

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"
	"webookApp/internal/usr/domain"

	"github.com/redis/go-redis/v9"
)

var ErrNotFind = redis.Nil

type UserCacher interface {
	Key(id int32) string
	SetProfile(ctx context.Context, u domain.User, id int32) error
	GetProfile(ctx context.Context, id int32) (domain.User, error)
}

type UserCache struct {
	RedisClient *redis.Client
}

func NewUserRedisCache(redisClient *redis.Client) *UserCache {
	return &UserCache{
		RedisClient: redisClient,
	}
}

func (r *UserCache) Key(id int32) string {
	return fmt.Sprintf("user:info:%d", id)
}

func (r *UserCache) SetProfile(ctx context.Context, u domain.ProfileResponde, id int32) error {
	key := r.Key(id)
	timeRand := rand.Intn(10) + 1 // 保存随机事件，简单解决缓存雪崩
	// 开启事务,保证原子性
	_, err := r.RedisClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		err := pipe.HSet(ctx, key, u).Err()
		if err != nil {
			return err
		}
		err = pipe.HExpire(ctx, key, time.Minute*time.Duration(timeRand), "nickname", "birthday", "aboutme", "followee", "follower").Err()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("user info HEXPIRE:%w", err)
	}
	return nil
}

func (r *UserCache) GetProfile(ctx context.Context, id int32) (domain.ProfileResponde, error) {
	key := r.Key(id)
	values, err := r.RedisClient.HVals(ctx, key).Result()
	if err != nil {
		return domain.ProfileResponde{}, err
	}
	length := len(values)
	if length == 0 {
		return domain.ProfileResponde{}, ErrNotFind
	}

	follower, _ := strconv.ParseInt(values[0], 10, 32)
	followee, _ := strconv.ParseInt(values[1], 10, 32)
	return domain.ProfileResponde{
		Follower: follower,
		Followee: followee,
		Nickname: values[2],
		Birthday: values[3],
		Aboutme:  values[4],
	}, nil
}

func (r *UserCache) GetFolloweeAndFollower(ctx context.Context, id int32) (int64, int64, error) {
	followingCountKey := fmt.Sprintf("follow:following:%v", id)
	followerCountKey := fmt.Sprintf("follow:count:%v", id)
	followingCount, errFollowing := r.RedisClient.SCard(ctx, followingCountKey).Result()
	followerCount, errFollower := r.RedisClient.Get(ctx, followerCountKey).Int64()

	if errFollower == redis.Nil {
		followerCount = 0
		errFollower = nil
	}
	if errFollowing == redis.Nil {
		errFollowing = nil
	}
	return followerCount, followingCount, errors.Join(errFollower, errFollowing)
}
