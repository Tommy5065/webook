package code

import (
	"context"
	"errors"
	"webookApp/internal/code"
	"webookApp/pkg/middelware/ratelimit"

	"go.uber.org/zap"
)

var (
	erroLimit error = errors.New("触发限流") // 内部错误不要暴露
)

type SMSServiceDecorator struct {
	limit  *ratelimit.RedisSlidingWindowLimiter // 限流具体参数类型
	svcSMS code.SDKService                      // 不同服务商实现的SMSService
}

// NewAlibabaSMSServiceDecorator 带有限流装饰器的SMSService服务
func NewSMSServiceDecorator(limiter *ratelimit.RedisSlidingWindowLimiter, svc code.SDKService) *SMSServiceDecorator {
	return &SMSServiceDecorator{
		limit:  limiter,
		svcSMS: svc,
	}
}

func (ra *SMSServiceDecorator) Send(ctx context.Context, args []string, phoneNumber string) error {
	res, err := ra.limit.Limit(ctx, "AlibabaSMS")
	if err != nil {
		// 说明限流装饰器出错，是否要继续把短信请求转发到下游
		// 1.如果下游实力很强，可以不限：业务可用性要求很高, 尽量容错策略
		// 2.如果下游实力不强，那就不转发到下游，避免系统雪花崩溃

		zap.L().Error("短信服务限流器出问题:", zap.Error(err))
		return ra.svcSMS.Send(ctx, args, phoneNumber)
	}
	if res { // 触发限流
		return erroLimit
	}
	return ra.svcSMS.Send(ctx, args, phoneNumber)
}
