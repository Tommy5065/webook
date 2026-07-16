package web

import (
	"net/http"
	"webookApp/internal/follow/domain"
	"webookApp/internal/follow/service"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"

	"github.com/gin-gonic/gin"
)

type FollowHandler struct {
	srv *service.FollowService
}

func NewFollowHandler(service *service.FollowService) *FollowHandler {
	return &FollowHandler{
		srv: service,
	}
}

func (lh *FollowHandler) RegisterRoute(g *gin.Engine) {
	ag := g.Group("/follow")
	ag.POST("/add", lh.Follow)
	ag.POST("/cancel", lh.DisFollow)

}

func (lh *FollowHandler) Follow(ctx *gin.Context) {
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
		FolloweeID   int64  `json:"followee_id" binding:"required"`
		FollowerName string `json:"follower_name" binding:"required"`
	}

	var req Request
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusOK, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	c := ctx.Request.Context()
	totalNumber, err := lh.srv.Follow(c, req.FolloweeID, author.UserId, req.FollowerName)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, RespModel.Respond[domain.ResultResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: domain.ResultResponde{
				IsNew:       true,
				TotalNumber: totalNumber,
			},
		})
		return
	}

	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ResultResponde]{
		Code: 2,
		Msg:  "关注成功",
		Data: domain.ResultResponde{
			IsNew:       false,
			TotalNumber: totalNumber,
		},
	})
}

func (lh *FollowHandler) DisFollow(ctx *gin.Context) {
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
		FolloweeId int64 `json:"followee_id" binding:"required"`
	}

	var req Request
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusOK, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	totalNumber, err := lh.srv.DisFollow(ctx, req.FolloweeId, author.UserId)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, RespModel.Respond[domain.ResultResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: domain.ResultResponde{
				IsNew:       false,
				TotalNumber: totalNumber,
			},
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ResultResponde]{
		Code: 2,
		Msg:  "取消关注成功",
		Data: domain.ResultResponde{
			IsNew:       true,
			TotalNumber: totalNumber,
		},
	})
}
