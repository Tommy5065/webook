package web

import (
	"net/http"
	"time"
	codeDomain "webookApp/internal/code/domain"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"
	"webookApp/internal/usr/domain"
	"webookApp/internal/usr/service"

	"github.com/dlclark/regexp2"
	"github.com/gin-contrib/sessions"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

const (
	emailRegexPattern       = `[a-zA-Z0-9_-]+(\.[a-zA-Z0-9_-]+)+`
	passwordRegexPattern    = `^(?=.*[^\da-zA-Z\s]).{1,9}$`
	phoneNumberRegexPattern = `^1[3456789]\d{9}$`
	biz                     = "100002" // 不同的biz表示不同的短信模板，也可以代表不同的业务
)

var (
	FrequentOpetions = codeDomain.ErrCodeSendTooMany
	ErrFileSystem    = codeDomain.ErrSystemError
	ErrInvalidCode   = codeDomain.ErrCodeInvalid
)

type UserHandler struct {
	// 预编译校验速度更快
	service            *service.UserService
	emailRegeExp       *regexp2.Regexp
	passwordRegeExp    *regexp2.Regexp
	phoneNumberRegeExp *regexp2.Regexp
}

func NewUser(svc *service.UserService) *UserHandler {
	return &UserHandler{
		service:            svc,
		emailRegeExp:       regexp2.MustCompile(emailRegexPattern, regexp2.None),
		passwordRegeExp:    regexp2.MustCompile(passwordRegexPattern, regexp2.None),
		phoneNumberRegeExp: regexp2.MustCompile(phoneNumberRegexPattern, regexp2.None),
	}
}

func (c *UserHandler) RegisterRoutes(server *gin.Engine) {
	ug := server.Group("/users")
	ug.POST("/signup", c.Signup)
	ug.POST("/login", c.Login)
	ug.POST("/edit", c.Edit)
	ug.GET("/profile", c.Profile)
	ug.PUT("/login/send", c.SendLoginSMS)
	ug.POST("/login/SMS", c.LoginOrSignup)
	ug.GET("/refresh", c.getAccessToken)
	ug.GET("/logout", c.logOut)
}

// Signup 邮箱密码注册
func (c *UserHandler) Signup(ctx *gin.Context) {
	type SignupReq struct {
		Email           string `form:"email" binding:"required"`
		Password        string `form:"password" binding:"required"`
		ConfirmPassword string `form:"confirmPassword" binding:"required"`
	}
	var req SignupReq
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "请求参数错误",
			Data: err.Error(),
		})
		return
	}

	//校验请求
	isEmail, err := c.emailRegeExp.MatchString(req.Email)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	if !isEmail {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 4,
			Msg:  "email格式不正确",
		})
		return
	}

	isPassword, err := c.passwordRegeExp.MatchString(req.Password)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	if !isPassword {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 4,
			Msg:  "密码至少包含1位特殊字符",
		})
		return
	}

	if req.ConfirmPassword != req.Password {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 4,
			Msg:  "两次密码不一致",
		})
		return
	}

	// 数据库操作
	err = c.service.Signup(ctx, domain.User{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch err {
		case service.ErrUserDuplicateEmail:
			ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
				Code: 4,
				Msg:  "已注册",
				Data: service.ErrUserDuplicateEmail.Error(),
			})
			return
		case service.ErrUserTimeOut:
			ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
				Code: 5,
				Msg:  "超时请重试",
				Data: service.ErrUserTimeOut.Error(),
			})
			return
		default:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
				Code: 4,
				Msg:  "系统错误",
				Data: "系统错误",
			})
			return
		}
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "注册成功",
		Data: "注册成功",
	})

}

// Login 邮箱密码登录
func (c *UserHandler) Login(ctx *gin.Context) {

	type Login struct {
		Email    string `form:"email" binding:"required"`
		Password string `form:"password" binding:"required"`
	}

	var req Login
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "无效请求",
			Data: err.Error(),
		})
		return
	}
	user, err := c.service.Login(ctx, req.Email, req.Password)
	if err != nil {
		switch err {
		case service.ErrUserInvalidEmailOrPassword:
			ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
				Code: 4,
				Msg:  "无效请求",
				Data: service.ErrUserInvalidEmailOrPassword.Error(),
			})
			return
		case service.ErrUserTimeOut:
			ctx.AbortWithStatusJSON(http.StatusRequestTimeout, RespModel.Respond[string]{
				Code: 5,
				Msg:  "请求超时请重试",
				Data: service.ErrUserTimeOut.Error(),
			})
			return
		default:
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
				Code: 5,
				Msg:  "系统错误",
			})
			return
		}
	}

	accessToken, err := c.service.GenerateJWT(ctx.Request, user, false)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	refreshToken, err := c.service.GenerateJWT(ctx.Request, user, true)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	session := sessions.Default(ctx)
	session.Set("accessToken", accessToken)
	session.Set("refreshToken", refreshToken)
	if err := session.Save(); err != nil {
		zap.L().Sugar().Error("login set session err:", err.Error())
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "登录成功",
		Data: "WelCome",
	})
}

// Edit 编辑不敏感信息
func (c *UserHandler) Edit(ctx *gin.Context) {
	// 这里是不做敏感信息的修改的

	// 获取JWT的加密数据
	claims, _ := ctx.Get("accessToken")
	// 类型是Any,interface{}都要断言
	userClaim, _ := claims.(*authentication.UserClaim)

	type EditReq struct {
		Nickname     string `form:"nickname" binding:"required"`
		Birthday     string `form:"birthday" binding:"required"`
		Introduction string `form:"text" binding:"required"`
	}
	var edirReq EditReq

	if err := ctx.ShouldBind(&edirReq); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "无效请求",
			Data: err.Error(),
		})
		return
	}

	_, err := time.Parse(time.DateOnly, edirReq.Birthday)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 2,
			Msg:  "生日格式不对",
		})
		return
	}

	// 存数据库
	err = c.service.UpdateNonSensitiveInfo(ctx, domain.User{
		ID:       userClaim.UserId,
		Nickname: edirReq.Nickname,
		Birthday: edirReq.Birthday,
		Aboutme:  edirReq.Introduction,
	})
	if err != nil {
		if err == service.ErrUserTimeOut {
			ctx.AbortWithStatusJSON(http.StatusRequestTimeout, RespModel.Respond[string]{
				Code: 5,
				Msg:  "请求超时",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "修改成功",
	})
}

// Profile 查询不敏感信息
// 路径参数为空，则查自己JWT
// 不为空路径参数的用户ID为资源标识符
func (c *UserHandler) Profile(ctx *gin.Context) {
	type getProfileID struct {
		UsrId int32 `form:"user_id"`
	}

	var profileRequset getProfileID

	if err := ctx.ShouldBindQuery(&profileRequset); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "无效请求",
			Data: err.Error(),
		})
	}

	if profileRequset.UsrId == 0 {
		auth_, exists := ctx.Get("accessToken")
		if !exists {
			ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
				Code: 4,
				Msg:  "令牌不存在",
			})
			return
		}
		author, ok := auth_.(*authentication.UserClaim)
		if !ok {
			ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
				Code: http.StatusUnauthorized,
				Msg:  "令牌无效",
			})
			return
		}
		profileRequset.UsrId = author.UserId
	}

	// 数据库操作
	userProfile, err := c.service.Profile(ctx, profileRequset.UsrId)
	switch err {
	case service.ErrUserTimeOut:
		ctx.AbortWithStatusJSON(http.StatusRequestTimeout, RespModel.Respond[domain.ProfileResponde]{
			Code: 2,
			Msg:  "超时请重试",
			Data: userProfile,
		})
		return
	case service.ErrUserNotFound:
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[domain.ProfileResponde]{
			Code: 2,
			Msg:  "用户不存在",
			Data: userProfile,
		})
		return
	case nil:
		ctx.JSON(http.StatusOK, RespModel.Respond[domain.ProfileResponde]{
			Code: 2,
			Msg:  "查询成功",
			Data: userProfile,
		})
		return
	default:
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ProfileResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: userProfile,
		})
	}
}

// SendLoginSMS 发送短信验证码
func (c *UserHandler) SendLoginSMS(ctx *gin.Context) {
	// 校验手机号
	type sendLoginSMSReq struct {
		Phone_number string `form:"phone_number" binding:"required"`
	}

	var req sendLoginSMSReq
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "无效请求",
			Data: err.Error(),
		})
		return
	}
	//校验请求
	isPhone, err := c.phoneNumberRegeExp.MatchString(req.Phone_number)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	if !isPhone {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 2,
			Msg:  "手机号码格式不对",
		})
		return
	}

	go func() {
		c.service.Send(ctx, req.Phone_number)
	}()

	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "发送验证码成功",
	})
}

// LoginOrSignup 手机号存在登录/不存在注册新用户
func (c *UserHandler) LoginOrSignup(ctx *gin.Context) {

	type inputCode struct {
		PhoneNumber string `form:"phone_number" binding:"required"`
		InputCode   string `form:"input_code" binding:"required"`
	}
	var req inputCode

	if err := ctx.ShouldBind(&req); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 4,
			Msg:  "无效请求",
			Data: err.Error(),
		})
		return
	}

	isPhone, err := c.phoneNumberRegeExp.MatchString(req.PhoneNumber)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	if !isPhone {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 2,
			Msg:  "手机号码格式不对",
		})
		return
	}

	// 校验验证码
	res, err := c.service.Verify(ctx, req.PhoneNumber, req.InputCode)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	if !res {
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 2,
			Msg:  "验证码错误",
		})
		return
	}

	// 直接对数据库操作
	user, err := c.service.CreateOrFind(ctx, req.PhoneNumber)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	accessToken, err := c.service.GenerateJWT(ctx.Request, user, false)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	refreshToken, err := c.service.GenerateJWT(ctx.Request, user, true)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	session := sessions.Default(ctx)
	session.Set("accessToken", accessToken)
	session.Set("refreshToken", refreshToken)
	if err := session.Save(); err != nil {
		zap.L().Sugar().Error("login set session err:", err.Error())
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}

	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "登录/注册成功",
	})
}

// getAccessToken 使用Refresh_token刷新access_token
func (c *UserHandler) getAccessToken(ctx *gin.Context) {
	session := sessions.Default(ctx)
	refresh_ := session.Get("refreshToken")
	refresh, _ := refresh_.(string)
	if refresh == "" { // 说明登录接口生成refresh_token过程出错
		zap.L().Sugar().Error("refresh_token is empty string.")
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	// 验证refresh_token合法/过期
	validToken, claims, err := c.service.VerifyJWT(refresh)
	if err != nil || !validToken.Valid {
		zap.L().Sugar().Error("refreshToken illegal:", zap.Error(err))
		ctx.AbortWithStatusJSON(http.StatusOK, RespModel.Respond[string]{
			Code: 2,
			Msg:  "重新登录",
		})
		return
	}
	// 使用User-Agent 简单检测token是否泄露
	if claims.UserAgent != ctx.Request.UserAgent() {
		// 泄露发生安全问题
		// 日志记录
		zap.L().Warn("claims.UserAgent != ctx.Request.UserAgent,token leaked.")
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	user := domain.User{
		ID:       claims.UserId,
		Nickname: claims.Name,
	}
	accessToken, _ := c.service.GenerateJWT(ctx.Request, user, false)
	session.Set("accessToken", accessToken)
	if err := session.Save(); err != nil {
		zap.L().Sugar().Error("refresh_token set session err:", err.Error())
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
}

// logOut 退出登录
func (c *UserHandler) logOut(ctx *gin.Context) {
	session := sessions.Default(ctx)
	session.Clear()                               // 删除所有键值对
	session.Options(sessions.Options{MaxAge: -1}) // 删除session过期
	session.Save()
	ctx.SetCookie(
		"authentication",
		" ",
		-1,
		"/",
		"localhost",
		false,
		true,
	)
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 2,
		Msg:  "退出成功",
	})
}
