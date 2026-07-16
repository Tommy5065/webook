package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

//go:embed slide_window.lua
var luaSlideWindow string

// RedisSlidingWindowLimiter Redis 上的滑动窗口算法限流器实现
type RedisSlidingWindowLimiter struct {
	cmd *redis.Client
	// 窗口大小
	interval time.Duration
	// 阈值
	rate int
	// Interval 内允许 Rate 个请求
	// 1s 内允许 3000 个请求
}

type RedisSlidingWindowLimiterConfig struct {
	Interval time.Duration
	Rate     int
}

func (r *RedisSlidingWindowLimiter) Build(key string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ok, err := r.Limit(ctx, key)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, "System fail")
			zap.L().Fatal("generate uuid failed:", zap.Error(err))
		}
		if ok {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, "please try later.")
			return
		}
		ctx.Next()
	}
}

func (r *RedisSlidingWindowLimiter) Limit(ctx context.Context, key string) (bool, error) {
	uid, err := uuid.NewUUID()
	if err != nil {
		return false, fmt.Errorf("generate uuid failed %w", err)
	}
	return r.cmd.Eval(ctx, luaSlideWindow, []string{key},
		time.Duration(r.interval), int(r.rate), time.Now().UnixMilli(), uid.String()).Bool()
}

func NewRedisSlidingWindowLimiter(rds *redis.Client, cfg *RedisSlidingWindowLimiterConfig) *RedisSlidingWindowLimiter {
	return &RedisSlidingWindowLimiter{
		cmd:      rds,
		interval: cfg.Interval,
		rate:     cfg.Rate,
	}
}
