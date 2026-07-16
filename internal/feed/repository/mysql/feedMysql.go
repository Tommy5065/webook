package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"webookApp/internal/feed/domain"

	"go.uber.org/zap"
)

type Feeddao struct {
	db *sql.DB
}

func NewFeedDao(mysql *sql.DB) *Feeddao {
	return &Feeddao{
		db: mysql,
	}
}

func (fd *Feeddao) LikesCountList(ctx context.Context, limit int, cursor *domain.ListLikesCursor) (feedItems []domain.FeedArticleItem, retError error) {
	statement1 := fmt.Sprintf(`SELECT 
    a.artical_id,
    a.title,
    a.auth_name,
	a.cnt,
	a.ctime,
	a.utime
	FROM articles a
	ORDER BY a.cnt DESC, a.artical_id DESC
	LIMIT %v;`, limit)

	statement2 := fmt.Sprintf(`SELECT
	a.artical_id,
	a.title,
    a.auth_name,
	a.cnt,
	a.ctime,
	a.utime
	FROM articles a
	WHERE (a.cnt<?) OR (a.cnt=? AND a.artical_id<?)
	ORDER BY a.cnt DESC,a.artical_id DESC 
	LIMIT %v;`, limit)

	switch cursor {
	case nil:
		feedItems, retError = fd.articleBaserowQuery(ctx, statement1)
	default:
		feedItems, retError = fd.articleBaserowQuery(ctx, statement2, cursor.LikesCount, cursor.LikesCount, cursor.IdBefore)
	}
	return feedItems, retError
}

// 门面设计根据rds判断articlesOrFollows是那个部分从那个部分查询文章内容
// rds是true 说明articlesOrFollows是aticle_id切片
// rds是false 说明articlesOrFollows是followee_id切片
func (fd *Feeddao) ListByFollow(ctx context.Context, articlesOrFollows []int64, limit int, cursor *domain.ListFollowCursor, rds bool) (feedItems []domain.FeedArticleItem, retError error) {
	switch rds {
	case true:
		return fd.listByFollowFromRedis(ctx, articlesOrFollows)
	default:
		return fd.listByFollowFromMySQL(ctx, articlesOrFollows, limit, cursor)
	}
}

// 执行已命中的redis查询内容
func (fd *Feeddao) listByFollowFromRedis(ctx context.Context, articlesOrFollows []int64) (feedItems []domain.FeedArticleItem, retError error) {
	var placeHolder []string
	for i := 0; i < len(articlesOrFollows); i++ {
		placeHolder = append(placeHolder, "?")
	}

	// redis命中已经在redis里排序和offset了
	statement1 := fmt.Sprintf(`SELECT
	a.artical_id,
	a.title,
    a.auth_name,
	a.cnt,
	a.ctime,
	a.utime
	FROM articles a
	WHERE a.artical_id IN (%s)
	ORDER BY a.utime DESC,a.artical_id DESC;`, strings.Join(placeHolder, ","))

	anyArgs := make([]any, len(articlesOrFollows))
	for i, v := range articlesOrFollows {
		anyArgs[i] = v
	}
	feedItems, retError = fd.articleBaserowQuery(ctx, statement1, anyArgs...)
	return feedItems, retError
}

// 执行未命中的情况--根据关注列表查内容
func (fd *Feeddao) listByFollowFromMySQL(ctx context.Context, articlesOrFollows []int64, limit int, cursor *domain.ListFollowCursor) (feedItems []domain.FeedArticleItem, retError error) {
	var placeHolder []string
	for i := 0; i < len(articlesOrFollows); i++ {
		placeHolder = append(placeHolder, "?")
	}

	// redis未命中,查找关注列表,按照时间排序的分页情况
	statement2 := fmt.Sprintf(`SELECT
	a.artical_id,
	a.title,
    a.auth_name,
	a.cnt,
	a.ctime,
	a.utime
	FROM articles a
	WHERE a.auth_id IN (%s) AND ((a.utime < ?) OR (a.utime=? AND a.artical_id < ?)) 
	AND a.artical_status=1
	ORDER BY a.utime DESC,a.artical_id DESC
	LIMIT %d;`, strings.Join(placeHolder, ","), limit)

	anyArgs := make([]any, 0, len(articlesOrFollows)+3)
	for _, v := range articlesOrFollows {
		anyArgs = append(anyArgs, v)
	}

	anyArgs = append(anyArgs, cursor.TimeBefore, cursor.TimeBefore, cursor.IdBefore)
	feedItems, retError = fd.articleBaserowQuery(ctx, statement2, anyArgs...)
	return feedItems, retError
}

func (fd *Feeddao) articleBaserowQuery(ctx context.Context, query string, args ...any) (feedItems []domain.FeedArticleItem, retError error) {
	rows, err := fd.db.QueryContext(ctx, query, args...)
	if err != nil {
		zap.L().Error("feed的MYSQL操作出错", zap.Error(err))
		return feedItems, err
	}

	defer rows.Close()
	if rows.Err() != nil {
		zap.L().Error("feed的rows操作出错", zap.Error(err))
		return feedItems, rows.Err()
	}

	for rows.Next() {
		items := domain.FeedArticleItem{}
		if retError = rows.Scan(&items.ID, &items.Title, &items.Author, &items.LikesCount, &items.CreateTime, &items.UpdateTime); retError != nil {
			zap.L().Error("feed循环rows出错:", zap.Error(retError))
			return feedItems, retError
		}
		items.ReadUrl = "http://localhost:8090/article/" + strconv.FormatInt(items.ID, 10)
		feedItems = append(feedItems, items)
	}

	return feedItems, retError
}

// 查看用户关注列表
func (fd *Feeddao) ListOfFollows(ctx context.Context, usr int32) (followList []int64, retError error) {
	statement := `SELECT followee_id FROM follow_log WHERE follower_id=? AND status=1`
	rows, err := fd.db.QueryContext(ctx, statement, usr)
	if err != nil {
		zap.L().Error("查询usr的关注操作出错", zap.Error(err))
		return followList, err
	}

	defer rows.Close()
	if rows.Err() != nil {
		zap.L().Error("查询关注的rows出错", zap.Error(err))
		return followList, rows.Err()
	}

	for rows.Next() {
		var id int64
		if retError = rows.Scan(&id); retError != nil {
			zap.L().Error("关注循环rows出错:", zap.Error(retError))
			return followList, retError
		}
		followList = append(followList, id)
	}

	return followList, retError
}
