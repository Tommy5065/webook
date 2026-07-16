package articlewire

import (
	web "webookApp/internal/article/handler"
	"webookApp/internal/article/repository"
	"webookApp/internal/article/repository/cache"
	dao "webookApp/internal/article/repository/mysql"
	"webookApp/internal/article/service"
	"webookApp/pkg/oss"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var ArticleHandlerSet = wire.NewSet(
	dao.NewArticalDao,
	cache.NewArticalCache,
	repository.NewArticalRepository,
	NewOssClient,
	service.NewArticalService,
	web.NewArticalHander,
)

func NewOssClient(v *viper.Viper) *oss.OssServer {
	conf := oss.CreateOssServerConf{
		Endpoint:     v.GetString("minio.endpoint"),
		AccessKeyID:  v.GetString("minio.accessKeyID"),
		SecreteKeyID: v.GetString("minio.secretAccessKey"),
		UseSecure:    v.GetBool("minio.useSSL"),
	}
	return oss.NewOssServer(conf)
}
