package cache

import (
	"context"
	_ "embed"
	"fmt"
	scripts "webookApp/internal/code/repository/cache/lua"

	"github.com/redis/go-redis/v9"
)

type CodeCacher interface {
	key(phoneNumber, biz string) string
	Send(ctx context.Context, phoneNumber, code, biz string) (int, error)
	Verify(ctx context.Context, phoneNumber, inputCode, biz string) (int, error)
}

type CodeCache struct {
	Cmd *redis.Client
}

func NewCodeCache(rs *redis.Client) *CodeCache {
	return &CodeCache{
		Cmd: rs,
	}
}

func (rc *CodeCache) key(phoneNumber, biz string) string {
	// key设计成phone_number:login:131XXXXX914
	return fmt.Sprintf("phone_number:%s:%s", biz, phoneNumber)
}

// 执行设置验证码Lua脚本
func (rc *CodeCache) Send(ctx context.Context, phoneNumber, code, biz string) (int, error) {
	key := rc.key(phoneNumber, biz)
	return rc.Cmd.Eval(ctx, scripts.SetCodeLua, []string{key}, code).Int()

}

// 执行校验验证码lua脚本
func (rc *CodeCache) Verify(ctx context.Context, phoneNumber, inputCode, biz string) (int, error) {
	key := rc.key(phoneNumber, biz)
	return rc.Cmd.Eval(ctx, scripts.VerifyCode, []string{key}, inputCode).Int()
}
