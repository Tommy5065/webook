//go:build wireinject

package app

import (
	"database/sql"
	articleWeb "webookApp/internal/article/handler"
	bgService "webookApp/internal/background"
	feedWeb "webookApp/internal/feed/handler"
	followWeb "webookApp/internal/follow/handler"
	likeWeb "webookApp/internal/like/handler"
	sseWeb "webookApp/internal/sse"
	userWeb "webookApp/internal/usr/handler"
	articlewire "webookApp/wire/article"
	baseconwire "webookApp/wire/baseconfig"
	feedwire "webookApp/wire/feed"
	followwire "webookApp/wire/follow"
	likewire "webookApp/wire/like"
	ssewire "webookApp/wire/sse"
	usrwire "webookApp/wire/usr"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// App 包含所有组件
type App struct {
	Engine           *gin.Engine
	UserHandler      *userWeb.UserHandler
	ArticleHandler   *articleWeb.ArticalHandler
	LikeHandler      *likeWeb.LikeHandler
	FollowHandler    *followWeb.FollowHandler
	SseHandler       *sseWeb.SSEManagerHandler
	BackgroundSyncer bgService.BackgroundService // 或者单独包含 syncer
	Cleanup          func()                      // 组合所有清理函数
}

func NewApp(
	engine *gin.Engine,
	userHandler *userWeb.UserHandler,
	articleHandler *articleWeb.ArticalHandler,
	likeHandler *likeWeb.LikeHandler,
	followHandler *followWeb.FollowHandler,
	feedHandler *feedWeb.FeedHandler,
	sseHandler *sseWeb.SSEManagerHandler,
	bgApp bgService.BackgroundService,
	db *sql.DB,
	rds *redis.Client,

) *App {
	// 注册路由
	userHandler.RegisterRoutes(engine)
	articleHandler.RegisterRoute(engine)
	likeHandler.RegisterRoute(engine)
	followHandler.RegisterRoute(engine)
	feedHandler.RegisterRoute(engine)
	sseHandler.RegisterRoute(engine)

	// 构建清理函数
	cleanup := func() {
		for _, i := range bgApp {
			i.Stop()
		}
		db.Close()
		rds.Close()
	}
	return &App{
		Engine:           engine,
		UserHandler:      userHandler,
		ArticleHandler:   articleHandler,
		LikeHandler:      likeHandler,
		FollowHandler:    followHandler,
		SseHandler:       sseHandler,
		BackgroundSyncer: bgApp,
		Cleanup:          cleanup,
	}
}

// 顶层 wire 构建
func InitializeApp() (*App, error) {
	wire.Build(
		// 基础组件
		baseconwire.InitViper,
		baseconwire.InitMySQL,
		baseconwire.InitRedis,
		baseconwire.NewZapLogger,
		baseconwire.NewSessionStore,
		baseconwire.NewSessionJwt,
		baseconwire.NewRateLimiter,
		baseconwire.NewSnowFlake,
		baseconwire.NewGinEngine,

		// 各子模块 Provider Set
		usrwire.UserHandlerSet,
		articlewire.ArticleHandlerSet,
		likewire.LikeHandlerSet,
		followwire.FollowHandlerSet,
		feedwire.FeedHandlerSet,
		ssewire.SseSet,

		// background 服务
		bgService.ProvideLikeSyncer,
		bgService.ProviderSMSSyncer,
		bgService.ProvideFollowSyncer,
		bgService.ProvideLikeConsumer,
		bgService.ProvideFollowConsumer,
		bgService.ProvideBackgroundServices,

		// 最终 App
		NewApp,
	)
	return nil, nil
}
