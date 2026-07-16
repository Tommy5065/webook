package repository

import (
	"context"
	"webookApp/internal/article/domain"
	"webookApp/internal/article/repository/cache"
	dao "webookApp/internal/article/repository/mysql"
)

type ArticalRepository struct {
	ad  *dao.ArticalDao
	rds *cache.ArticleCache
}

func NewArticalRepository(adao *dao.ArticalDao, adcache *cache.ArticleCache) *ArticalRepository {
	return &ArticalRepository{
		ad:  adao,
		rds: adcache,
	}
}

func (ar *ArticalRepository) Add(ctx context.Context, ad domain.Artical) error {
	return ar.ad.Insert(ctx, ad)
}

func (ar *ArticalRepository) Publish(ctx context.Context, ad domain.Artical, status domain.ArticalStatus) error {
	return ar.ad.Upsert(ctx, ad, status)
}

func (ar *ArticalRepository) UpdateStatus(ctx context.Context, ad domain.Artical, status domain.ArticalStatus) error {
	return ar.ad.UpdateStatus(ctx, status, ad)
}

func (ar *ArticalRepository) CheckWithAuthor(ctx context.Context, articleID int64, userID int32) (error, domain.ArticleSearchRespond) {
	return ar.ad.SearchWithAuthor(ctx, articleID, userID)
}

func (ar *ArticalRepository) Check(ctx context.Context, articleID int64) (error, domain.ArticleSearchRespond) {
	return ar.ad.Search(ctx, articleID)
}

func (ar *ArticalRepository) List(ctx context.Context, userID int32, limit, offset int64) (error, []domain.ArticleSearchRespond) {
	return ar.ad.SearchList(ctx, userID, limit, offset)
}

func (ar *ArticalRepository) SearchFanList(ctx context.Context, followeeID int32) (error, []int32) {
	return ar.ad.SearchFanList(ctx, followeeID)
}

func (ar *ArticalRepository) InsertBox(ctx context.Context, ids []int32, articleID, timeStamp int64) error {
	return ar.rds.InBox(ctx, ids, articleID, timeStamp)
}
