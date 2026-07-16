package repository

import (
	"context"
	dao "webookApp/internal/async/repository/mysql"
	"webookApp/internal/code/domain"
)

type AsyncRepositoryer interface {
	Add(ctx context.Context, asyncSms domain.SmsEntity) error
	ReportScheduleResult(ctx context.Context, id int64, success bool) error
	PreemptWaitingSMS(ctx context.Context) (domain.SmsEntity, error)
}

type AsyncRepository struct {
	daoAsync *dao.AsyncDao
}

var ErrorWaitingNotFoundSMS error = dao.ErrorWaitingNotFoundSMS

func NewAsyncRepository(daoAsync *dao.AsyncDao) *AsyncRepository {
	return &AsyncRepository{
		daoAsync: daoAsync,
	}
}

func (ar *AsyncRepository) Add(ctx context.Context, asyncSms *domain.SmsEntity) error {
	return ar.daoAsync.Insert(ctx, dao.Async{
		PhoneNumber: asyncSms.PhoneNumber,
		TplID:       asyncSms.TplID,
		Code:        asyncSms.Code,
	})
}

func (ar *AsyncRepository) ReportScheduleResult(ctx context.Context, id int64, success bool) error {
	if success {
		return ar.daoAsync.MarkSuccess(ctx, id)
	}
	return ar.daoAsync.MarkFail(ctx, id)
}

func (ar *AsyncRepository) PreemptWaitingSMS(ctx context.Context) (*domain.SmsEntity, error) {
	as, err := ar.daoAsync.Search(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.SmsEntity{
		ID:          as.ID,
		PhoneNumber: as.PhoneNumber,
		Code:        as.Code,
		TplID:       as.TplID,
	}, nil
}
