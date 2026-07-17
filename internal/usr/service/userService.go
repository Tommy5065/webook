package service

import (
	"context"
	"net/http"
	"time"
	"webookApp/internal/code/service"
	"webookApp/internal/middelware/authentication"
	"webookApp/internal/usr/domain"
	"webookApp/internal/usr/repository"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	code   *service.CodeService
	repo   *repository.UserRepository
	usrJwt *authentication.SessionJwt
}

func NewUserService(r *repository.UserRepository, c *service.CodeService, j *authentication.SessionJwt) *UserService {
	return &UserService{
		repo:   r,
		code:   c,
		usrJwt: j,
	}
}

var (
	ErrUserDuplicateEmail         = repository.ErrUserDuplicateEmail
	ErrUserTimeOut                = repository.ErrUserTimeout
	ErrUserInvalidEmailOrPassword = repository.ErrUserInvalidEmailOrPassword
	ErrRedisNotFind               = repository.ErrRedisNotFind
	ErrUserPhoneInvalid           = repository.ErrUserPhoneInvalid
	ErrUserNotFound               = repository.ErrUserNotFound
)

const biz string = "login"

func (srv *UserService) Signup(ctx context.Context, u domain.User) error {
	// 加密放在service层
	hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	u.Password = string(hash)

	// recall repository loade data
	err := srv.repo.Create(ctx, u)
	switch err {
	case ErrUserDuplicateEmail:
		return ErrUserDuplicateEmail
	case ErrUserTimeOut:
		return ErrUserTimeOut
	default:
		return err
	}
}

func (srv *UserService) Login(ctx context.Context, email, password string) (user domain.User, err error) {
	// 先找用户
	user, err = srv.repo.FindByEmail(ctx, email)
	if err != nil {
		switch err {
		case ErrUserInvalidEmailOrPassword:
			return user, ErrUserInvalidEmailOrPassword
		case ErrUserTimeOut:
			return user, ErrUserTimeOut
		default:
			return user, err
		}
	}

	// 对比密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return user, ErrUserInvalidEmailOrPassword
	}
	return user, nil
}

func (srv *UserService) UpdateNonSensitiveInfo(ctx context.Context, u domain.User) error {
	err := srv.repo.UpdateByID(ctx, u)
	if err != nil {
		switch err {
		case ErrUserTimeOut:
			return ErrUserTimeOut
		default:
			return err
		}
	}
	return nil
}

func (srv *UserService) Profile(ctx context.Context, id int32) (user domain.ProfileResponde, err error) {
	// 加入缓存优化性能
	// 注意未解决缓存三大问题
	user, err = srv.repo.RedisFindByID(ctx, id)

	// 如果是缓存本身有问题,是否应该让大量请求打入Mysql，兜底方案
	switch err {
	case nil:
		return user, err
	default:
	}

	if err == ErrRedisNotFind {
		user, errMysql := srv.repo.FindByID(ctx, id)
		switch errMysql {
		case ErrUserTimeOut:
			return user, ErrUserTimeOut
		case ErrUserNotFound:
			return user, ErrUserNotFound
		case nil:
			updateErr := srv.repo.RedisSet(ctx, user, id)
			if updateErr != nil {
				zap.L().Error("redis set info err:", zap.Error(err))
			}
			return user, nil
		default:
			return user, errMysql
		}
	}
	return user, err
}

func (srv *UserService) CreateOrFind(ctx context.Context, phoneNumber string) (domain.User, error) {
	// 快查询:数据库读的速度比写的速度快
	user, err := srv.repo.FindByPhone(ctx, phoneNumber)
	if err != nil {
		switch err {
		case ErrUserPhoneInvalid:
			// 说明电话号码不存在，创建新用户，但是比较粗糙用户可能有两个号：1个邮箱注册1个手机注册
			// 慢查询：数据库写操作
			if err = srv.repo.CreateByPhoneNumber(ctx, phoneNumber); err != nil {
				return user, err
			}
		case ErrUserTimeOut:
			return user, ErrUserTimeOut
		default:
			return user, err
		}
	}

	return user, err
}

func (srv *UserService) Send(ctx context.Context, phoneNumber string) {
	srv.code.Send(ctx, biz, phoneNumber)
}

func (srv *UserService) Verify(ctx context.Context, phoneNumber, inputCode string) (bool, error) {
	return srv.code.Verify(ctx, biz, phoneNumber, inputCode)
}

func (srv *UserService) GenerateJWT(ctx *http.Request, user domain.User, refresh bool) (string, error) {
	tokenClaim := srv.user2claim(ctx, user, refresh)
	return srv.usrJwt.GenerateJwt(tokenClaim)
}

func (srv *UserService) VerifyJWT(token string) (*jwt.Token, *authentication.UserClaim, error) {
	return srv.usrJwt.Verify(token)
}

func (srv *UserService) user2claim(c *http.Request, usr domain.User, refresh bool) *authentication.UserClaim {
	// 生成tokenClaims,传入payload加密,accesstoken有效期5分钟
	// freshtoken有效期7天
	var expiresTime *jwt.NumericDate
	if refresh {
		expiresTime = jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))
	} else {
		expiresTime = jwt.NewNumericDate(time.Now().Add(5 * time.Minute))
	}

	tokenClaims := authentication.UserClaim{
		UserId: usr.ID,
		Name:   usr.Nickname,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expiresTime,
		},
		UserAgent: c.UserAgent(),
	}
	return &tokenClaims
}
