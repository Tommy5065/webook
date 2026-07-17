package authentication

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	_ "encoding/hex"
	"encoding/pem"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type AuthJWTer interface {
	AuthJWTIgnore(path string) *AuthJWT
	AuthenBuild() gin.HandlerFunc
	GenRsaKey(bits int) (*AuthJWT, error)
	Verify(tokenstr string) (*jwt.Token, *UserClaim, error)
	GenerateJwt(claim jwt.Claims) (string, error)
}

type AuthJWT struct {
	path        []string `wire:"-"`
	privateFile []byte
	publicFile  []byte
}

// 自定义Claim接口
type UserClaim struct {
	jwt.RegisteredClaims // 嵌入满足接口结构体
	UserId               int32
	Name                 string
	UserAgent            string
}

func NewAuthJWT(bits int) (*AuthJWT, error) {
	//Generates private key.
	privateFile, err := os.ReadFile("privateKey.pem")
	if err != nil {
		return nil, err
	}

	// Generate public key
	publicFile, err := os.ReadFile("publicKey.pem")
	if err != nil {
		return nil, err
	}

	return &AuthJWT{
		publicFile:  publicFile,
		privateFile: privateFile,
	}, nil

}

func (JWT *AuthJWT) AuthJWTIgnore(path string) *AuthJWT {
	JWT.path = append(JWT.path, path)
	return JWT
}

func (JWT *AuthJWT) AuthenBuild() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, v := range JWT.path {
			if v == ctx.Request.URL.Path {
				ctx.Next()
				return
			}
		}

		// 验证token是否存在/正确
		auth := ctx.Request.Header.Get("Authorization")
		if auth == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, "please login")
			return
		}
		token := strings.SplitN(auth, " ", 2)
		if len(token) != 2 { // 说明脚本请求，可能是攻击
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, "please login")
			return
		}

		tokStr := token[1]
		validToken, claims, err := JWT.Verify(tokStr)
		if err != nil || !validToken.Valid {
			zap.L().Error("Verify token false:", zap.Error(err))
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, "please login")
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

		// 中间件从token里获取Claims
		ctx.Set("claims", claims)
		ctx.Next()
	}

}

func (JWT *AuthJWT) GenerateJwt(claim jwt.Claims) (string, error) {
	//使用RS需要生成公私钥

	// 注意这里传入值，不是指针，使用值副本生成对应的token而不是修改结构体
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claim)

	// 使用对应私钥加密
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(JWT.privateFile)
	if err != nil {
		zap.L().Fatal("get private key:", zap.Error(err))
		return "", err
	}

	tokStr, err := token.SignedString(privateKey)
	if err != nil {
		zap.L().Fatal("generate final jwt:", zap.Error(err))
		return "", err
	}
	return tokStr, nil
}

func (JWT *AuthJWT) Verify(tokenstr string) (*jwt.Token, *UserClaim, error) {
	// 注意这里使用指针ParseWithClaims把解析到的数据通过反射写入结构体,写入应该传入指针
	claims := &UserClaim{}

	// 回调函数动态获取正确密钥,确保密钥符合head指定算法
	token, err := jwt.ParseWithClaims(tokenstr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwt.ParseRSAPublicKeyFromPEM(JWT.publicFile)
	})
	if err != nil {
		return nil, claims, err
	}

	return token, claims, nil
}

func (JWT *AuthJWT) GenRsaKey(bits int) error {
	//Generates private key.
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}

	// 封装私钥
	x509Private := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509Private,
	}
	privateFile, err := os.Create("privateKey.pem")
	if err != nil {
		return err
	}
	defer privateFile.Close()
	err = pem.Encode(privateFile, block)
	if err != nil {
		return err
	}

	// Generate public key
	x509Public, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	block = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509Public,
	}

	publicFile, err := os.Create("publicKey.pem")
	if err != nil {
		return err
	}
	defer publicFile.Close()
	err = pem.Encode(publicFile, block)
	if err != nil {
		return err
	}

	return nil

}
