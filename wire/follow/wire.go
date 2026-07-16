package followwire

import (
	"webookApp/internal/follow/domain"
	web "webookApp/internal/follow/handler"
	followCache "webookApp/internal/follow/repository/cache"
	dao "webookApp/internal/follow/repository/mysql"
	service_ "webookApp/internal/follow/service"
	mq "webookApp/internal/mq/redmq"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var FollowHandlerSet = wire.NewSet(
	// MQ consumer/producer 构建
	followCache.NewFollowCache,
	dao.NewFollowDao,
	NewFollowConsumerConfig,
	NewFollowProducer,
	service_.NewFollowService,
	web.NewFollowHandler,
)

// 从 viper 读取配置构建 consumerConfig 和 retryConsumerConfig
func NewFollowConsumerConfig(v *viper.Viper, rds *redis.Client) *domain.FollowConsumerAllConfig {
	cfg := &mq.MqConsumerConfig{
		RedisClient: rds,
		BuffSize:    v.GetInt("mq.common.buffSize"),
		StreamName:  v.GetString("mq.follow.streamName"),
		GroupName:   v.GetString("mq.follow.groupName"),
		Department:  v.GetString("mq.follow.department"),
		Count:       v.GetInt64("mq.common.count"),
		Block:       v.GetDuration("mq.common.block"),
		Timer:       v.GetDuration("mq.common.timer"),
	}
	// 还要有重试配置，同理读取
	retryCfg := &mq.MqRetryConsumerConfig{
		RetryTimer: v.GetDuration("mq.follow.retryTimer"),
		Idle:       v.GetDuration("mq.follow.idle"),
		RetryTime:  v.GetInt64("mq.follow.retryTime"),
		Count:      v.GetInt64("mq.common.count"),
	}

	return &domain.FollowConsumerAllConfig{
		NormalConfig: cfg,
		RetryConfig:  retryCfg,
	}
}

func NewFollowProducer(v *viper.Viper, rds *redis.Client) *domain.FollowProducer {
	cfg := &mq.MqProducerConfig{
		Redis:          rds,
		Topic:          v.GetString("mq.follow.streamName"),
		NoMkStream:     true,
		MaxLen:         v.GetInt64("mq.common.maxLen"),
		Approx:         v.GetBool("mq.common.approx"),
		Limit:          v.GetInt64("mq.common.limit"),
		ProducerId:     v.GetString("mq.follow.department"), // 或自定义
		IdempotentAuto: true,
	}
	return &domain.FollowProducer{
		Producer: mq.NewMqProducer(cfg),
	}
}
