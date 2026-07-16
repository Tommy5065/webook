package service

import (
	"context"
	"fmt"
	"math/rand/v2"

	"webookApp/internal/code/domain"
	code "webookApp/internal/code/rateLimiterSMS"
	"webookApp/internal/code/repository/cache"
	dao "webookApp/internal/code/repository/mysql"

	"go.uber.org/zap"
)

type CodeService struct {
	codeCache   *cache.CodeCache
	codeDao     *dao.CodeDao
	sdkDecorate *code.SMSServiceDecorator
}

func NewCodeService(
	codeCache *cache.CodeCache,
	codeDao *dao.CodeDao,
	sdkDecorate *code.SMSServiceDecorator,
) *CodeService {
	return &CodeService{
		codeCache:   codeCache,
		codeDao:     codeDao,
		sdkDecorate: sdkDecorate,
	}
}

// Send 生成验证码并存入 Redis，然后将发送任务写入 MySQL 队列
func (svc *CodeService) Send(ctx context.Context, biz string, phoneNumber string) error {
	// 1. 生成验证码
	code := fmt.Sprintf("%06d", rand.IntN(1000000))

	// 2. 存储验证码到 Redis (使用 Lua 脚本保证原子性)
	// 返回值 0 通常代表成功，-1 代表频繁，-2 代表系统错误
	// 使用带限流器的SDK发送保护第三方
	res, err := svc.codeCache.Send(ctx, phoneNumber, code, biz)
	if err != nil || res == -2 {
		return domain.ErrSystemError
	}

	// 检查是否发送过于频繁 (Lua 脚本返回 -1 表示频繁)
	if res == -1 {
		return domain.ErrCodeSendTooMany
	}

	tplId := svc.getBizTemplateId(biz)

	asyncSms := domain.SmsEntity{
		PhoneNumber: phoneNumber,
		Code:        code,
		TplID:       tplId,
	}

	// 发送短信，出错了将任务插入数据库换别的SKD/本SDK重试发送
	if err = svc.sdkDecorate.Send(ctx, []string{asyncSms.Code, "5分钟", asyncSms.TplID}, asyncSms.PhoneNumber); err != nil {
		err = svc.codeDao.Insert(ctx, asyncSms)
		if err != nil {
			zap.L().Error("写入短信队列失败", zap.Error(err))
			return domain.ErrSystemError
		}
	}
	return nil
}

// Verify 校验验证码
func (svc *CodeService) Verify(ctx context.Context, biz, phone, inputCode string) (bool, error) {
	res, err := svc.codeCache.Verify(ctx, phone, inputCode, biz)
	if err != nil {
		return false, domain.ErrSystemError
	}
	// 假设 Lua 返回 0 为成功，-2 为失败
	if res == -1 {
		zap.L().Warn("有攻击者调用短信验证:", zap.String("号码:", phone))
	}
	return res == 0, nil
}

// getBizTemplateId 根据业务类型获取模板 ID
func (svc *CodeService) getBizTemplateId(biz string) string {
	switch biz {
	case "login":
		return "100001"
	case "register":
		return "100002"
	default:
		return "001"
	}
}
