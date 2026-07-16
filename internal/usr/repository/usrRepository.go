package repository

import (
	"context"
	"errors"
	"webookApp/internal/usr/domain"
	"webookApp/internal/usr/repository/cache"
	dao "webookApp/internal/usr/repository/mysql"
)

type UserRepositoryer interface {
	Create(ctx context.Context, u domain.User) error
	CreateByPhoneNumber(ctx context.Context, phoneNumber string) error
	FindByEmail(ctx context.Context, email string) (user domain.User, err error)
	UpdateByID(ctx context.Context, u domain.User) error
	FindByID(ctx context.Context, id int32) (user *domain.ProfileResponde, err error)
	FindByPhone(ctx context.Context, phoneNumber string) (user domain.User, err error)
	RedisFindByID(ctx context.Context, id int32) (domain.User, error)
	RedisSet(ctx context.Context, u domain.User, id int32) error
}

type UserRepository struct {
	daoUser   *dao.UserDao
	usercache *cache.UserCache
}

var (
	ErrUserDuplicateEmail         = dao.ErrUserEmailDuplicate
	ErrUserTimeout                = dao.ErrUserTimeOut
	ErrUserInvalidEmailOrPassword = dao.ErrUserInvalidEmailOrPassword
	ErrRedisNotFind               = cache.ErrNotFind
	ErrUserPhoneInvalid           = dao.ErrUserInvalidPhone
)

func (rt *UserRepository) Create(ctx context.Context, u domain.User) error {
	err := rt.daoUser.Insert(ctx, dao.User{
		Email:    u.Email,
		Password: u.Password,
	})
	switch err {
	case ErrUserDuplicateEmail:
		return ErrUserDuplicateEmail
	case ErrUserTimeout:
		return ErrUserTimeout
	default:
		return err
	}
}

func (rt *UserRepository) CreateByPhoneNumber(ctx context.Context, phoneNumber string) error {
	err := rt.daoUser.InsertByPhone(ctx, dao.User{
		PhoneNumber: phoneNumber,
	})
	switch err {
	case ErrUserTimeout:
		return ErrUserTimeout
	default:
		return err
	}
}

func (rt *UserRepository) FindByEmail(ctx context.Context, email string) (user domain.User, err error) {
	user, err = rt.daoUser.SearchUserByEmail(ctx, email)
	if err != nil {
		switch err {
		case ErrUserInvalidEmailOrPassword:
			return user, ErrUserInvalidEmailOrPassword
		case ErrUserTimeout:
			return user, ErrUserTimeout
		default:
			return user, err
		}
	}
	return user, err
}

func (rt *UserRepository) UpdateByID(ctx context.Context, u domain.User) error {
	err := rt.daoUser.UpdateByID(ctx, dao.User{
		Id:       u.ID,
		NickName: u.Nickname,
		Birthday: u.Birthday,
		AboutMe:  u.Aboutme,
	})

	if err != nil {
		switch err {
		case ErrUserTimeout:
			return ErrUserTimeout
		default:
			return err
		}
	}
	return nil
}

func (rt *UserRepository) FindByID(ctx context.Context, id int32) (user domain.ProfileResponde, err error) {
	user, errInfo := rt.daoUser.Profile(ctx, id)
	return user, errors.Join(errInfo, errInfo)
}

func (rt *UserRepository) FindByPhone(ctx context.Context, phoneNumber string) (user domain.User, err error) {
	user, err = rt.daoUser.SearchUserByPhone(ctx, phoneNumber)
	if err != nil {
		switch err {
		case ErrUserPhoneInvalid:
			return user, ErrUserPhoneInvalid
		case ErrUserTimeout:
			return user, ErrUserTimeout
		default:
			return user, err
		}
	}
	return user, err
}

func (rt *UserRepository) RedisFindByID(ctx context.Context, id int32) (domain.ProfileResponde, error) {
	user, err := rt.usercache.GetProfile(ctx, id)
	if err != nil {
		switch err {
		case ErrRedisNotFind:
			return user, ErrRedisNotFind
		default:
			return user, err
		}
	}
	return user, err
}

func (rt *UserRepository) RedisSet(ctx context.Context, u domain.ProfileResponde, id int32) error {
	err := rt.usercache.SetProfile(ctx, u, id)
	if err != nil {
		return err
	}
	return err
}

func NewUserRepository(daoUser *dao.UserDao, rc *cache.UserCache) *UserRepository {
	return &UserRepository{
		daoUser:   daoUser,
		usercache: rc,
	}
}
