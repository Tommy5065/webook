package web

import (
	"net/http"
	"webookApp/internal/like/domain"
	"webookApp/internal/like/service"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	srv *service.LikeService
}

func NewLikeHandler(service *service.LikeService) *LikeHandler {
	return &LikeHandler{
		srv: service,
	}
}

func (lh *LikeHandler) RegisterRoute(g *gin.Engine) {
	ag := g.Group("/like")
	ag.POST("/add", lh.Like)
	ag.POST("/cancel", lh.DisLike)

}

func (lh *LikeHandler) Like(ctx *gin.Context) {
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: http.StatusUnauthorized,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[int64]{
			Code: http.StatusUnauthorized,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type Request struct {
		ArticalId     int64  `json:"article_id" binding:"required"`
		ArticleUserID int32  `json:"article_user_id" binding:"required"`
		Title         string `json:"title" binding:"required"`
		UserName      string `json:"user_name" binding:"required"`
	}

	var req Request
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}
	// 读取底部context上下文
	// 在ServerHTTP返回,客户端断开连接,HTTP/2 取消被控制的协程会退出
	c := ctx.Request.Context()
	totalNumber, err := lh.srv.Like(c, req.ArticalId, author.UserId, req.ArticleUserID, req.Title, req.UserName)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, RespModel.Respond[domain.ResultResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: domain.ResultResponde{
				IsNew:       true, // 点赞不成功还是未点赞状态
				TotalNumber: totalNumber,
			},
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ResultResponde]{
		Code: 2,
		Msg:  "点赞成功",
		Data: domain.ResultResponde{
			IsNew:       false, // 已点赞状态
			TotalNumber: totalNumber,
		},
	})
}

func (lh *LikeHandler) DisLike(ctx *gin.Context) {
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: http.StatusUnauthorized,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[int64]{
			Code: http.StatusUnauthorized,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type Request struct {
		ArticalId int64 `json:"article_id" binding:"required"`
	}

	var req Request
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	totalNumber, err := lh.srv.DisLike(ctx, req.ArticalId, author.UserId)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, RespModel.Respond[domain.ResultResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: domain.ResultResponde{
				TotalNumber: totalNumber, // 默认isNew是false 取消点赞失败还是已点赞状态
			},
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ResultResponde]{
		Code: 5,
		Msg:  "取消点赞成功",
		Data: domain.ResultResponde{
			IsNew:       true, // 变成未点赞状态了
			TotalNumber: totalNumber,
		},
	})
}
