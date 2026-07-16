package domain

import "errors"

type SmsEntity struct {
	ID          int64
	PhoneNumber string
	Code        string
	TplID       string
}

// 定义错误码，用于区分业务错误和系统错误
var (
	ErrCodeSendTooMany = errors.New("发送过于频繁")
	ErrCodeInvalid     = errors.New("验证码错误")
	ErrSystemError     = errors.New("系统错误")
)
