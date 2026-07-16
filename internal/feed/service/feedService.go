package service

import (
	"context"
	"webookApp/internal/feed/domain"
	"webookApp/internal/feed/repository/cache"
	dao "webookApp/internal/feed/repository/mysql"
)

type FeedService struct {
	feedDao   *dao.Feeddao
	feedCache *cache.FeedCache
}

func NewFeedService(fd *dao.Feeddao, fc *cache.FeedCache) *FeedService {
	return &FeedService{
		feedDao:   fd,
		feedCache: fc,
	}
}

func (fs *FeedService) ListLikesCount(ctx context.Context, limit int, cursor *domain.ListLikesCursor) (likesRespond domain.ListLikesResponde, retError error) {
	feedItems, err := fs.feedDao.LikesCountList(ctx, limit, cursor)
	if err != nil {
		return likesRespond, err
	}
	likesRespond = fs.buildLikeResponde(feedItems, limit)
	return likesRespond, retError
}

func (fs *FeedService) ListFollows(ctx context.Context, usrID int32, limit int, cursor *domain.ListFollowCursor) (followRespond domain.ListFollowResponde, retError error) {
	// 获取feed流文章id列表
	feedArticleID, errID := fs.feedCache.GetFollowList(ctx, usrID, limit)
	if feedArticleID == nil || errID != nil { // 说明未命中或者出错查数据库
		followList, err := fs.feedDao.ListOfFollows(ctx, usrID)
		if err != nil {
			return domain.ListFollowResponde{}, err
		}

		feedItems, errItem := fs.feedDao.ListByFollow(ctx, followList, limit, cursor, false)
		if errItem != nil {
			return domain.ListFollowResponde{}, errItem
		}
		followRespond := fs.buildFollowResponde(feedItems, limit)
		return followRespond, retError
	} else { // 命中直接查内容
		feedItems, errItem := fs.feedDao.ListByFollow(ctx, feedArticleID, limit, cursor, true)
		if errItem != nil {
			return domain.ListFollowResponde{}, errItem
		}
		followRespond := fs.buildFollowResponde(feedItems, limit)
		return followRespond, retError
	}
}

func (fs *FeedService) buildLikeResponde(feedItems []domain.FeedArticleItem, limit int) (listFeedResponde domain.ListLikesResponde) {
	if len(feedItems) == 0 {
		return listFeedResponde
	}
	if len(feedItems) == limit {
		listFeedResponde.HasMore = true
	}

	listFeedResponde.ArticleList = feedItems
	listFeedResponde.LikeCountsBefore = &feedItems[len(feedItems)-1].LikesCount
	listFeedResponde.IdBefore = &feedItems[len(feedItems)-1].ID
	return listFeedResponde
}

func (fs *FeedService) buildFollowResponde(feedItems []domain.FeedArticleItem, limit int) (listFollowResponde domain.ListFollowResponde) {
	if len(feedItems) == 0 {
		return listFollowResponde
	}

	listFollowResponde.ArticleList = feedItems
	listFollowResponde.LastTimeBefore = &feedItems[len(feedItems)-1].UpdateTime
	listFollowResponde.IdBefore = &feedItems[len(feedItems)-1].ID
	return listFollowResponde
}
