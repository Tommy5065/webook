package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type AsyncDaoer interface {
	Insert(ctx context.Context, async Async) error
	Search(ctx context.Context) (res Async, err error)
	MarkSuccess(ctx context.Context, id int64) error
	MarkFail(ctx context.Context, id int64) error
}

type AsyncDao struct {
	mysqlDB *sql.DB
}

var ErrorWaitingNotFoundSMS = fmt.Errorf("sms_fail tuple not found. ")

func NewAsync(MysqlDB *sql.DB) *AsyncDao {
	return &AsyncDao{
		mysqlDB: MysqlDB,
	}
}

type Async struct {
	ID          int64
	PhoneNumber string
	Code        string
	TplID       string
}

func (ac *AsyncDao) Insert(ctx context.Context, async Async) error {
	statement := "INSERT INTO sms_fail(phone_number,templated_id,code) VALUES(?,?,?)"
	stmt, err := ac.mysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("INSERT sms_fail prepare fail:", zap.Error(err))
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, async.PhoneNumber, async.TplID, async.Code)
	if err != nil {
		zap.L().Error("INSERT sms_fail execute fail:", zap.Error(err))
		return err
	}
	return err
}

func (ac *AsyncDao) Search(ctx context.Context) (res Async, err error) {
	now := time.Now().UTC()                                            // 时区转换
	endTime := now.Truncate(time.Minute).Format("2006-01-02 15:04:05") // 找到前一分钟且转化成字符串
	tx, err := ac.mysqlDB.BeginTx(ctx, &sql.TxOptions{                 // 开启事务
		Isolation: sql.LevelDefault,
	})
	if err != nil {
		return res, fmt.Errorf("create a transaction fail:%w", err)
	}
	// 如果在高并发情况下,SELECT for UPDATE(行锁排他锁) 对数据库的压力很大
	// 但是我们不是高并发，因为你部署N台机器，才有 N 个goroutine 来查询
	// 并发不过百，随便写
	statement1 := "SELECT id,phone_number,code,templated_id from sms_fail WHERE utime < ? AND is_success = 0 FOR UPDATE;" //查询并锁住符合条件的行
	statement2 := "UPDATE sms_fail SET utime = ? WHERE id=?"
	err = tx.QueryRowContext(ctx, statement1, endTime).Scan(&res.ID, &res.PhoneNumber, &res.Code, &res.TplID)
	if err != nil {
		tx.Rollback()
		if err == sql.ErrNoRows {
			return res, ErrorWaitingNotFoundSMS // 找不到也会报错,标记一下
		}
		return res, fmt.Errorf("search sms err:%w", err)
	}
	// 更新时间，防止别的节点来抢,也相当于重试间隔1分钟
	_, err = tx.ExecContext(ctx, statement2, now.Format("2006-01-02 15:04:05"), res.ID)
	if err != nil {
		tx.Rollback()
		return res, fmt.Errorf("update utime err%w", err)
	}
	if err = tx.Commit(); err != nil {
		return res, fmt.Errorf("execute commit err:%w", err)
	}
	return res, err
}

func (ac *AsyncDao) MarkSuccess(ctx context.Context, id int64) error {
	statment := "UPDATE sms_fail SET is_success = 1 WHERE id = ?"
	stmt, err := ac.mysqlDB.Prepare(statment)
	if err != nil {
		return fmt.Errorf("markSuccressPrepare err:%w", err)
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("markSuccressExecute err:%w", err)
	}
	return nil
}

func (ac *AsyncDao) MarkFail(ctx context.Context, id int64) error {
	statment := "UPDATE sms_fail SET is_success = -1 WHERE id = ?"
	stmt, err := ac.mysqlDB.Prepare(statment)
	if err != nil {
		return fmt.Errorf("MarkFail Prepare err:%w", err)
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("MarkFail Execute err:%w", err)
	}
	return nil
}
