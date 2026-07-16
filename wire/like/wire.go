package likewire

import (
	"webookApp/internal/like/domain"
	web "webookApp/internal/like/handler"
	likecache "webookApp/internal/like/repository/cache"
	dao "webookApp/internal/like/repository/mysql"
	service_ "webookApp/internal/like/service"
	mq "webookApp/internal/mq/redmq"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var LikeHandlerSet = wire.NewSet(
	NewLikeConsumerConfig,
	NewLikeProducer,
	likecache.NewLikeCache,
	dao.NewLikeDao,
	service_.NewLikeService,
	web.NewLikeHandler,
)

// 从 viper 读取配置构建 consumer的配置
func NewLikeConsumerConfig(v *viper.Viper, rds *redis.Client) *domain.LikeMqConsumerAllConfig {
	cfg := &mq.MqConsumerConfig{
		RedisClient: rds,
		BuffSize:    v.GetInt("mq.common.buffSize"),
		StreamName:  v.GetString("mq.like.streamName"),
		GroupName:   v.GetString("mq.like.groupName"),
		Department:  v.GetString("mq.like.department"),
		Count:       v.GetInt64("mq.common.count"),
		Block:       v.GetDuration("mq.common.block"),
		Timer:       v.GetDuration("mq.common.timer"),
	}
	// 还要有重试配置，同理读取
	retryCfg := &mq.MqRetryConsumerConfig{
		RetryTimer: v.GetDuration("mq.like.retryTimer"),
		Idle:       v.GetDuration("mq.like.idle"),
		RetryTime:  v.GetInt64("mq.like.retryTime"),
		Count:      v.GetInt64("mq.like.count"),
	}
	return &domain.LikeMqConsumerAllConfig{
		NormalConfig: cfg,
		RetryConfig:  retryCfg,
	}
}

// 从 viper 读取配置构建 producer
func NewLikeProducer(v *viper.Viper, rds *redis.Client) *domain.LikeProducer {
	cfg := &mq.MqProducerConfig{
		Redis:          rds,
		Topic:          v.GetString("mq.like.streamName"),
		NoMkStream:     true,
		MaxLen:         v.GetInt64("mq.common.maxLen"),
		Approx:         v.GetBool("mq.common.approx"),
		Limit:          v.GetInt64("mq.common.limit"),
		ProducerId:     v.GetString("mq.like.department"), // 或自定义
		IdempotentAuto: true,
	}
	return &domain.LikeProducer{Producer: mq.NewMqProducer(cfg)}
}
