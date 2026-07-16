package authentication

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SessionJwter interface {
	AuthenBuild() gin.HandlerFunc
	IgnoreAuthPath(path string) *AuthenSession
}

type SessionJwt struct {
	path     []string // 有的路由不需要sessionID检查
	*AuthJWT          // 依赖注入,依靠authJWT的方法验证和生成JWT
}

func (auth *SessionJwt) AuthenBuild() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, v := range auth.path {
			if v == ctx.Request.URL.Path {
				ctx.Next()
				return
			}
		}

		session := sessions.Default(ctx)
		accessToken_ := session.Get("accessToken")
		if accessToken_ == nil { // 说明没有走登录
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 301,
				"msg":  "please login.",
			})
			return
		}

		// 验证accessToken是否合法/过期
		accessToken, _ := accessToken_.(string)
		if accessToken == "" {
			zap.L().Sugar().Error("accessToken is empty string.")
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code": 501, // 内部错误码,后续会补充完整的一套程序错误码
				"msg":  "system error.",
			})
			return
		}
		validToken, claims, err := auth.Verify(accessToken)
		if err != nil || !validToken.Valid {
			zap.L().Sugar().Error("accessToken illegal:", zap.Error(err))
			ctx.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code": 302,
				"msg":  "please change accessToken.",
			})
			return
		}
		// 使用User-Agent 简单检测token是否泄露
		if claims.UserAgent != ctx.Request.UserAgent() {
			// 泄露发生安全问题
			// 日志记录
			zap.L().Error("claims.UserAgent != ctx.Request.UserAgent,token leaked.")
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		ctx.Set("accessToken", claims)
		ctx.Next()
	}

}

func (auth *SessionJwt) IgnoreAuthPath(path string) *SessionJwt {
	auth.path = append(auth.path, path)
	return auth
}

func NewSessionJWT(Jwt *AuthJWT) *SessionJwt {
	return &SessionJwt{
		AuthJWT: Jwt,
	}
}
