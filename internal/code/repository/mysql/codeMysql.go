package dao

import (
	"context"
	"database/sql"
	"fmt"

	"webookApp/internal/code/domain"

	"go.uber.org/zap"
)

type CodeDao struct {
	mysqlDB *sql.DB
}

var ErrorWaitingNotFoundSMS = fmt.Errorf("sms_fail tuple not found. ")

func NewCodeDao(MysqlDB *sql.DB) *CodeDao {
	return &CodeDao{
		mysqlDB: MysqlDB,
	}
}

func (ac *CodeDao) Insert(ctx context.Context, param domain.SmsEntity) error {
	statement := "INSERT INTO sms_fail(phone_number,templated_id,code) VALUES(?,?,?)"
	stmt, err := ac.mysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("INSERT sms_fail prepare fail:", zap.Error(err))
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, param.PhoneNumber, param.TplID, param.Code)
	if err != nil {
		zap.L().Error("INSERT sms_fail execute fail:", zap.Error(err))
		return err
	}
	return err
}
