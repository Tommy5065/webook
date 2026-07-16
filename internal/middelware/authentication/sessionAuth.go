package authentication

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Seesioner interface {
	AuthenBuild() gin.HandlerFunc
	IgnoreAuthPath(path string) *AuthenSession
}

type AuthenSession struct {
	path []string // 有的路由不需要sessionID检查
}

func (auth *AuthenSession) AuthenBuild() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, v := range auth.path {
			if v == ctx.Request.URL.Path {
				ctx.Next()
				return
			}
		}

		session := sessions.Default(ctx)
		if session.Get("auth") == nil {
			zap.L().Sugar().Info("Unauthorized")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, "please login.")
		}
		ctx.Next()
	}

}

func (auth *AuthenSession) IgnoreAuthPath(path string) *AuthenSession {
	auth.path = append(auth.path, path)
	return auth
}

func NewAuthenSession() *AuthenSession {
	return &AuthenSession{}
}
