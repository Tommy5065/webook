package mq

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// 消息队列的生产者,完善这个生产者该有的配置
type MqProducer struct {
	redisCmd       *redis.Client
	topic          string
	noMkStream     bool
	maxLen         int64 // 消息队列最大长度
	approx         bool  // 是严格=最大长度还是近似~
	limit          int64 // 超过了最大长度一次剪枝多少
	producerId     string
	idempotentAuto bool // 设置幂等
}

type MqProducerConfig struct {
	Redis          *redis.Client
	Topic          string
	NoMkStream     bool
	MaxLen         int64 // 消息队列最大长度
	Approx         bool  // 是严格=最大长度还是近似~
	Limit          int64 // 超过了最大长度一次剪枝多少
	ProducerId     string
	IdempotentAuto bool // 设置幂等
}

func NewMqProducer(config *MqProducerConfig) *MqProducer {
	return &MqProducer{
		redisCmd:       config.Redis,
		noMkStream:     config.NoMkStream,
		topic:          config.Topic,
		maxLen:         config.MaxLen,
		approx:         config.Approx,
		limit:          config.Limit,
		producerId:     config.ProducerId,
		idempotentAuto: config.IdempotentAuto,
	}
}

// 生产者放入消息
func (lr *MqProducer) Add(ctx context.Context, values []interface{}) (string, error) {
	// stream名称不能为空
	if lr.topic == "" {
		return "", errors.New("topic is empty")
	}

	return lr.redisCmd.XAdd(ctx, &redis.XAddArgs{
		Stream:         lr.topic,
		NoMkStream:     lr.noMkStream,
		MaxLen:         lr.maxLen,
		Approx:         lr.approx,
		ID:             "*",
		Limit:          lr.limit,
		Values:         values,
		ProducerID:     lr.producerId,
		IdempotentAuto: lr.idempotentAuto,
	}).Result()
}
