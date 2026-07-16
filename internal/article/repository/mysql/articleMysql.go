package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"webookApp/internal/article/domain"

	"go.uber.org/zap"
)

type ArticalDao struct {
	mysqlDB *sql.DB
}

func NewArticalDao(MysqlDB *sql.DB) *ArticalDao {
	return &ArticalDao{
		mysqlDB: MysqlDB,
	}
}

func (a *ArticalDao) Insert(ctx context.Context, ad domain.Artical) error {
	statement1 := "INSERT INTO draft(draft_id,title,content,auth_id,ctime,utime) VALUES(?,?,?,?,?,?);"
	_, err := a.mysqlDB.ExecContext(ctx, statement1, ad.ArtiID, ad.Title, ad.Content, ad.Author.ID, ad.Ctime, ad.Utime)
	if err != nil {
		zap.L().Error("新建文章出错", zap.Error(err))
		return err
	}
	return nil
}

func (a *ArticalDao) Upsert(ctx context.Context, ad domain.Artical, status domain.ArticalStatus) (err error) {
	// 开启事务
	tx, err := a.mysqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// 保证最终提交或回滚
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	var affectNumber int64

	if ad.New {
		// 新建草稿
		stmt := "INSERT INTO draft(draft_id,title,content,auth_id,ctime,utime) VALUES(?,?,?,?,?,?)"
		_, err = tx.ExecContext(ctx, stmt, ad.ArtiID, ad.Title, ad.Content, ad.Author.ID, ad.Ctime, ad.Utime)
		if err != nil {
			zap.L().Error("发表新建文章失败:", zap.Error(err))
			return err
		}
	} else {
		// 更新草稿，同时验证 auth_id
		stmt := "UPDATE draft SET title=?, content=?,utime=? WHERE draft_id=? AND auth_id=?"
		result, _ := tx.ExecContext(ctx, stmt, ad.Title, ad.Content, ad.Utime, ad.ArtiID, ad.Author.ID)
		affectNumber, err = result.RowsAffected()
		if err != nil {
			zap.L().Error("发表更新文章失败:", zap.Error(err))
			return err
		}

		if affectNumber == 0 {
			// 没有匹配的草稿，说明越权或文章不存在，回滚事务
			zap.L().Warn("有攻击者篡改文章", zap.Int32("攻击者ID:", ad.Author.ID), zap.Error(err))
			return fmt.Errorf("有攻击者篡改文章")
		}
	}

	// 幂等更新 articles 表
	stmt := `INSERT INTO articles(artical_id,title,content,auth_name,auth_id,ctime,utime) 
             VALUES(?,?,?,?,?,?,?) 
             ON DUPLICATE KEY UPDATE 
			 	artical_id=VALUES(artical_id),
                title=VALUES(title), 
                content=VALUES(content),
                auth_name=VALUES(auth_name),
				auth_id=VALUES(auth_id),
				ctime = IF(VALUES(ctime) > 0, VALUES(ctime), ctime),
				utime = VALUES(utime)`
	_, err = tx.ExecContext(ctx, stmt, ad.ArtiID, ad.Title, ad.Content, ad.Author.Name, ad.Author.ID, ad.Ctime, ad.Utime)
	if err != nil {
		zap.L().Error("更新artical表失败:", zap.Error(err))
		return err
	}
	// 写回draft表状态
	stmt2 := `UPDATE draft SET draft_status=?,utime=? WHERE draft_id=? AND auth_id=?`
	_, err = tx.ExecContext(ctx, stmt2, status, time.Now().UTC().UnixMilli(), ad.ArtiID, ad.Author.ID)
	if err != nil {
		zap.L().Error("写回draft表状态失败:", zap.Error(err))
		return err
	}
	return err
}

func (a *ArticalDao) UpdateStatus(ctx context.Context, status domain.ArticalStatus, ad domain.Artical) error {
	tx, err := a.mysqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// 保证最终提交或回滚
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	statement1 := "UPDATE draft SET draft_status=?,utime=? WHERE draft_id=? AND auth_id=?;"
	statement2 := "UPDATE articles SET artical_status=?,utime=? WHERE artical_id=?;"
	res, err := tx.ExecContext(ctx, statement1, status, ad.Utime, ad.ArtiID, ad.Author.ID)
	if err != nil {
		zap.L().Error("更改draft表的draft_status字段执行错误:", zap.Error(err))
		return err
	}

	affectNumber, err := res.RowsAffected()
	if err != nil {
		zap.L().Error("更新draft状态失败:", zap.Error(err))
		return err
	}
	if affectNumber == 0 {
		zap.L().Warn("有攻击者修改文章状态", zap.Int32("攻击者ID:", ad.Author.ID), zap.Error(err))
		return fmt.Errorf("有攻击者删除文章")
	}
	_, err = tx.ExecContext(ctx, statement2, status, ad.Utime, ad.ArtiID)
	if err != nil {
		zap.L().Error("更新articles表状态失败:", zap.Error(err))
		return err
	}
	return err
}

func (a *ArticalDao) Search(ctx context.Context, articelID int64) (error, domain.ArticleSearchRespond) {
	result := domain.ArticleSearchRespond{}
	statement := "SELECT title,content,auth_name,cnt,ctime,utime FROM articles WHERE artical_id=? AND artical_status=1"
	err := a.mysqlDB.QueryRowContext(ctx, statement, articelID).Scan(&result.Title, &result.Content, &result.AuthorInfo.Name, &result.LikeCount, &result.CreateTime, &result.UpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, result
		}
		zap.L().Error("查看文章内容失败:", zap.Error(err))
		return err, result
	}
	return nil, result
}

func (a *ArticalDao) SearchList(ctx context.Context, userID int32, limit, offset int64) (error, []domain.ArticleSearchRespond) {
	statement := "SELECT draft_id,title,cnt,ctime,utime FROM draft WHERE auth_id=? ORDER BY utime DESC LIMIT ? OFFSET ?"

	var reslutArrary []domain.ArticleSearchRespond
	rows, err := a.mysqlDB.QueryContext(ctx, statement, userID, limit, offset)
	if err != nil {
		return err, reslutArrary
	}
	defer rows.Close()

	if rows.Err() != nil {
		zap.L().Error("查看文章列表row出错:", zap.Error(err))
		return rows.Err(), reslutArrary
	}

	for rows.Next() {
		result := domain.ArticleSearchRespond{}
		if err := rows.Scan(&result.ID, &result.Title, &result.LikeCount, &result.CreateTime, &result.UpdateTime); err != nil {
			if err == sql.ErrNoRows {
				return nil, reslutArrary
			}
			zap.L().Error("循环文章列表rows出错:", zap.Error(err))
			return err, reslutArrary
		}
		reslutArrary = append(reslutArrary, result)
	}

	return nil, reslutArrary
}

func (a *ArticalDao) SearchWithAuthor(ctx context.Context, articelID int64, userID int32) (error, domain.ArticleSearchRespond) {
	result := domain.ArticleSearchRespond{}
	statement := "SELECT title,content,cnt,ctime,utime FROM draft WHERE draft_id=? AND auth_id=?"
	err := a.mysqlDB.QueryRowContext(ctx, statement, articelID, userID).Scan(&result.Title, &result.Content, &result.LikeCount, &result.CreateTime, &result.UpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, result
		}
		zap.L().Error("作者查看自己文章详情", zap.Error(err))
		return err, result
	}
	return nil, result
}

// 发表文章成功，查询粉丝id--为更新粉丝收件箱做准备
func (a *ArticalDao) SearchFanList(ctx context.Context, followeeID int32) (retError error, fanlist []int32) {
	statement := "SELECT follower_id FROM follow_log WHERE followee_id=? AND status=1;"
	rows, err := a.mysqlDB.QueryContext(ctx, statement, followeeID)
	if err != nil {
		zap.L().Error("开启查询粉丝ID列表Rows失败:", zap.Error(err))
		return err, fanlist
	}
	defer rows.Close()

	if rows.Err() != nil {
		zap.L().Error("查询粉丝ID获得的Rows存在错误:", zap.Error(err))
		return rows.Err(), fanlist
	}

	for rows.Next() {
		var id int32
		retError = rows.Scan(&id)
		switch retError {
		case nil:
			fanlist = append(fanlist, id)
		case sql.ErrNoRows:
			return retError, fanlist
		default:
			zap.L().Error("查询粉丝ID循环Rows未知错误:", zap.Error(retError))
			return retError, fanlist
		}
	}
	return retError, fanlist

}
