package usrwire

import (
	"webookApp/internal/code"
	"webookApp/internal/code/SDK/memory"
	codeSDK "webookApp/internal/code/rateLimiterSMS"
	codeCache "webookApp/internal/code/repository/cache"
	codeDao "webookApp/internal/code/repository/mysql"
	codeService "webookApp/internal/code/service"
	web "webookApp/internal/usr/handler"
	"webookApp/internal/usr/repository"
	"webookApp/internal/usr/repository/cache"
	dao "webookApp/internal/usr/repository/mysql"
	"webookApp/internal/usr/service"
	"webookApp/pkg/middelware/ratelimit"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var UserHandlerSet = wire.NewSet(
	// 基础 repo 和 service
	cache.NewUserRedisCache,
	dao.NewUserDao,
	repository.NewUserRepository,
	service.NewUserService,

	// SMS 相关
	memory.NewSmsMemory,
	wire.Bind(new(code.SDKService), new(*memory.SmsMemory)),
	codeCache.NewCodeCache,
	codeDao.NewCodeDao,
	NewRateLimit,
	codeSDK.NewSMSServiceDecorator,
	codeService.NewCodeService,
	// 最终 handler
	web.NewUser,
)

type smsLimit *ratelimit.RedisSlidingWindowLimiter

func NewRateLimit(v *viper.Viper, rds *redis.Client) smsLimit {
	cfg := &ratelimit.RedisSlidingWindowLimiterConfig{
		Interval: v.GetDuration("rateLimit.sms.interval"),
		Rate:     v.GetInt("rateLimit.sms.rate"),
	}
	return ratelimit.NewRedisSlidingWindowLimiter(rds, cfg)
}
