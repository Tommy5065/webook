package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"webookApp/internal/usr/domain"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

type UserDaoer interface {
	Insert(ctx context.Context, user User) error
	SearchUserByEmail(ctx context.Context, email string) (domain.User, error)
	UpdateByID(ctx context.Context, user User) error
	Profile(ctx context.Context, id int32) (domain.User, error)
	SearchUserByPhone(ctx context.Context, phoneNumber string) (domain.User, error)
	InsertByPhone(ctx context.Context, user User) error
}

type UserDao struct {
	MysqlDB *sql.DB
}

type User struct {
	Id       int32
	Email    string
	Password string
	// 存毫秒，返回给前端格式化
	Ctime       int64
	Utime       int64
	NickName    string
	Birthday    string
	AboutMe     string
	PhoneNumber string
}

var (
	ErrUserEmailDuplicate         = errors.New("email has exitd.")
	ErrUserTimeOut                = errors.New("Time Out!")
	ErrUserInvalidEmailOrPassword = errors.New("email/password wrong.")
	ErrUserInvalidPhone           = errors.New("phoneNumber wrong.")
	ErrUserNotFound               = sql.ErrNoRows
)

func NewUserDao(db *sql.DB) *UserDao {
	return &UserDao{
		MysqlDB: db,
	}
}

func (ud *UserDao) Insert(ctx context.Context, user User) error {
	time := time.Now().UnixMilli()
	statement := "INSERT INTO users(email,password,ctime,utime) VALUES(?,?,?,?)"
	stmt, err := ud.MysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("Mysql:", zap.String("insert prepare", err.Error()))
		return err
	}
	defer stmt.Close()

	// 带有超时的上下文控制
	_, err = stmt.ExecContext(ctx, user.Email, user.Password, time, time)
	if err != nil {
		if err == context.DeadlineExceeded {
			zap.L().Error("Mysql insert:", zap.Error(ErrUserTimeOut))
			return ErrUserTimeOut
		}

		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			const uniqueConflictsErr uint16 = 1062
			if mysqlErr.Number == uniqueConflictsErr {
				zap.L().Error("Mysql insert", zap.Error(ErrUserEmailDuplicate))
				return ErrUserEmailDuplicate
			}
		}
	}
	return nil
}

func (ud *UserDao) SearchUserByEmail(ctx context.Context, email string) (domain.User, error) {
	res := domain.User{}
	statement := "SELECT id,email,password,nickname FROM users WHERE email=?"
	stmt, err := ud.MysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("Mysql:", zap.String("searchUser prepare", err.Error()))
		return res, err
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, email).Scan(&res.ID, &res.Email, &res.Password, &res.Nickname)
	if err != nil {
		switch err {
		case context.DeadlineExceeded:
			zap.L().Error("Mysql insert:", zap.Error(ErrUserTimeOut))
			return res, ErrUserTimeOut
		case sql.ErrNoRows:
			zap.L().Error("Mysql insert:", zap.Error(err))
			return res, ErrUserInvalidEmailOrPassword
		default:
			zap.L().Error("Mysql insert:", zap.Error(err))
			return res, err
		}

	}
	return res, nil
}

func (ud *UserDao) UpdateByID(ctx context.Context, user User) error {
	time := time.Now().UnixMilli()
	statement := "UPDATE users SET nickname=?,birthday=?,aboutme=?,utime=? WHERE id=?;"
	stmt, err := ud.MysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("Mysql:", zap.String("Mysql UpdateByID prepare", err.Error()))
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, user.NickName, user.Birthday, user.AboutMe, time, user.Id)
	if err != nil {
		if err == context.DeadlineExceeded {
			zap.L().Error("Mysql UpdateById execute:", zap.Error(ErrUserTimeOut))
			return ErrUserTimeOut
		}
		zap.L().Error("Mysql:", zap.String("Mysql UpdateByID execute", err.Error()))
		return err
	}
	return nil
}

func (ud *UserDao) Profile(ctx context.Context, id int32) (domain.ProfileResponde, error) {
	profile := domain.ProfileResponde{}
	statement1 := "SELECT u.nickname,u.birthday,u.aboutme,u.fan_count,u.following_count FROM users u WHERE u.id = ?;"
	retError := ud.MysqlDB.QueryRowContext(ctx, statement1, id).Scan(&profile.Nickname, &profile.Birthday, &profile.Aboutme, &profile.Follower, &profile.Followee)
	switch retError {
	case sql.ErrNoRows:
		return profile, ErrUserNotFound
	case ErrUserTimeOut:
		return profile, ErrUserTimeOut
	case nil:
		return profile, nil
	default:
		zap.L().Error("查询profile出错:", zap.Error(retError))
		return profile, retError
	}
}

func (ud *UserDao) SearchUserByPhone(ctx context.Context, phoneNumber string) (domain.User, error) {
	res := domain.User{}
	statement := "SELECT id,nickname FROM users WHERE phone_number=?"
	stmt, err := ud.MysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("Mysql:", zap.String("Mysql SearchUserByPhone prepare", err.Error()))
		return res, err
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, phoneNumber).Scan(&res.ID, &res.Nickname)
	if err != nil {
		switch err {
		case context.DeadlineExceeded:
			zap.L().Error("Mysql SearchUserByPhone query:", zap.Error(ErrUserTimeOut))
			return res, ErrUserTimeOut
		case sql.ErrNoRows:
			return res, ErrUserInvalidPhone
		default:
			zap.L().Error("Mysql SearchUserByPhone query:", zap.Error(err))
			return res, err
		}
	}
	return res, nil
}

func (ud *UserDao) InsertByPhone(ctx context.Context, user User) error {
	time := time.Now().UnixMilli()
	statement := "INSERT INTO users(phone_number,password,ctime,utime) VALUES(?,?,?,?)"
	stmt, err := ud.MysqlDB.Prepare(statement)
	if err != nil {
		zap.L().Error("Mysql:", zap.String("insertByPhone prepare", err.Error()))
		return err
	}
	defer stmt.Close()

	// 带有超时的上下文控制
	_, err = stmt.ExecContext(ctx, user.PhoneNumber, user.Password, time, time)
	if err != nil {
		if err == context.DeadlineExceeded {
			zap.L().Error("Mysql insertByPhone:", zap.Error(ErrUserTimeOut))
			return ErrUserTimeOut
		}
		return err
	}
	return nil
}
