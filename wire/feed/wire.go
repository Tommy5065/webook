package feedwire

import (
	web "webookApp/internal/feed/handler"
	"webookApp/internal/feed/repository/cache"
	dao "webookApp/internal/feed/repository/mysql"
	"webookApp/internal/feed/service"

	"github.com/google/wire"
)

var FeedHandlerSet = wire.NewSet(
	dao.NewFeedDao,
	cache.NewFeedCache,
	service.NewFeedService,
	web.NewFeedHandler,
)
