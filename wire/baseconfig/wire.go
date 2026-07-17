package baseconwire

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"webookApp/internal/middelware/authentication"
	"webookApp/internal/middelware/logger"
	"webookApp/pkg/middelware/ratelimit"
	"webookApp/pkg/snowflake"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ---------- 基础 Provider ----------
func InitViper() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath("./wire/baseconfig/")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		zap.L().Sugar().Info("config changed")
	})
	return v, nil
}

func InitMySQL(v *viper.Viper) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC",
		v.GetString("mysql.user"),
		v.GetString("mysql.password"),
		v.GetString("mysql.host"),
		v.GetInt("mysql.port"),
		v.GetString("mysql.dbName"))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(v.GetInt("mysql.maxOpenConns"))
	db.SetMaxIdleConns(v.GetInt("mysql.maxIdleConns"))
	if err := db.PingContext(context.Background()); err != nil {
		return nil, err
	}
	return db, nil
}

func InitRedis(v *viper.Viper) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", v.GetString("redis.host"), v.GetInt("redis.port")),
		DB:           8,
		Username:     "webook",
		PoolSize:     v.GetInt("redis.poolSize"),
		MaxIdleConns: v.GetInt("redis.maxIdleConns"),
		MinIdleConns: v.GetInt("redis.minIdleConns"),
	})
	return client
}

// ---------- 日志、中间件、路由 ----------
func NewZapLogger(v *viper.Viper) (*logger.ZapLogger, error) {
	return logger.NewZapLogger(v)
}

func NewGinEngine(
	zapLogger *logger.ZapLogger,
	v *viper.Viper,
	rateLimiter *ratelimit.RedisSlidingWindowLimiter,
	sessionStore sessions.Store,
	jwt *authentication.SessionJwt,
) *gin.Engine {
	r := gin.New()
	r.Use(zapLogger.GinRecover(false))
	r.Use(zapLogger.GinLogger())
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"PUT", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length", "X-JWT:", "Set-Cookie"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "http://localhost")
		},
		MaxAge: 12 * time.Hour,
	}))
	r.Use(rateLimiter.Build("IP"))
	r.Use(sessions.Sessions("authentication", sessionStore))
	r.Use(jwt.AuthenBuild()) // 但需要提前设置 IgnoreAuthPath
	return r
}

// SessionStore Provider
func NewSessionStore(v *viper.Viper) sessions.Store {
	store := cookie.NewStore(
		[]byte(v.GetString("app.signatureKey")),
		[]byte(v.GetString("app.encryptKey")),
	)
	store.Options(sessions.Options{
		MaxAge:   v.GetInt("session.maxAge"),
		HttpOnly: true,
		Domain:   "localhost",
		Path:     "/",
	})
	return store
}

// 认证中间件（提供已配置好忽略路径的 SessionJwt）
func NewSessionJwt(v *viper.Viper) (*authentication.SessionJwt, error) {
	bits := v.GetInt("jwt.bits")
	auth, err := authentication.NewAuthJWT(bits)
	if err != nil {
		return nil, err
	}
	sjwt := authentication.NewSessionJWT(auth)
	// 配置忽略路径（可从 viper 读取列表，这里直接硬编码，也可配置在 yaml）
	sjwt.IgnoreAuthPath("/users/signup").
		IgnoreAuthPath("/users/login").
		IgnoreAuthPath("/users/login/send").
		IgnoreAuthPath("/users/login/SMS").
		IgnoreAuthPath("/users/refresh").
		IgnoreAuthPath("/users/logout").
		IgnoreAuthPath("/feed/list/likes")
	return sjwt, nil
}

// 限流器 Provider
func NewRateLimiter(v *viper.Viper, rds *redis.Client) *ratelimit.RedisSlidingWindowLimiter {
	cfg := &ratelimit.RedisSlidingWindowLimiterConfig{
		Interval: v.GetDuration("rateLimit.ip.interval"),
		Rate:     v.GetInt("rateLimit.ip.rate"),
	}
	return ratelimit.NewRedisSlidingWindowLimiter(rds, cfg)
}

// 雪花算法 Provider
func NewSnowFlake(v *viper.Viper) snowflake.SnowFlaker {
	epoch := v.GetString("app.time")
	machineID := v.GetInt64("snowflake.machineID")
	return snowflake.NewSnowFlakeSetting(epoch, machineID)
}
