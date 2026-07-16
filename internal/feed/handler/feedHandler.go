package web

import (
	"net/http"
	"webookApp/internal/feed/domain"
	"webookApp/internal/feed/service"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	srv *service.FeedService
}

func NewFeedHandler(feedService *service.FeedService) *FeedHandler {
	return &FeedHandler{
		srv: feedService,
	}
}

func (fl *FeedHandler) RegisterRoute(g *gin.Engine) {
	ag := g.Group("/feed")
	ag.POST("/list/likes", fl.ListByLikesCount)
	ag.POST("/list/follow", fl.ListByFollow)
}

func (fl *FeedHandler) ListByLikesCount(ctx *gin.Context) {
	type ListLikesCountRequest struct {
		Limit            int    `json:"limit" binding:"required"` // int类型使用required不能传0，绑定认不出是未传值还是值是0
		LikeCountsBefore *int64 `json:"likes_counts_before,omitempty"`
		IdBefore         *int64 `json:"likes_id_before,omitempty"`
	}
	var likesRequest ListLikesCountRequest
	if err := ctx.ShouldBindJSON(&likesRequest); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ListLikesResponde]{
			Code: 4,
			Msg:  "绑定错误",
			Data: domain.ListLikesResponde{},
		})
		return
	}
	if likesRequest.Limit > 50 {
		likesRequest.Limit = 10
	}

	var cursor *domain.ListLikesCursor

	// 判断无效游标
	if likesRequest.LikeCountsBefore != nil || likesRequest.IdBefore != nil {
		if likesRequest.LikeCountsBefore == nil || likesRequest.IdBefore == nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ListLikesResponde]{
				Code: 4,
				Msg:  "likeCountsBefore和idBefore需要一起传",
				Data: domain.ListLikesResponde{},
			})
			return
		}

		likesCountsBefore := *likesRequest.LikeCountsBefore
		idBefore := *likesRequest.IdBefore

		if idBefore < 0 {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ListLikesResponde]{
				Code: 4,
				Msg:  "idBefore 必须大于0",
				Data: domain.ListLikesResponde{},
			})
			return
		} else {
			cursor = &domain.ListLikesCursor{
				LikesCount: likesCountsBefore,
				IdBefore:   idBefore,
			}
		}
	}

	feedItem, err := fl.srv.ListLikesCount(ctx, likesRequest.Limit, cursor)

	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, RespModel.Respond[domain.ListLikesResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: domain.ListLikesResponde{},
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ListLikesResponde]{
		Code: 200,
		Msg:  "查找成功",
		Data: feedItem,
	})
}

func (fl *FeedHandler) ListByFollow(ctx *gin.Context) {
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
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: http.StatusUnauthorized,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type ListFollowRequest struct {
		Limit      int   `json:"limit" binding:"required"`
		LatestTime int64 `json:"latest_time" binding:"required"` // 前端至少要传今天的日期时间戳
		IdBefore   int64 `json:"id_before"`
	}

	var followRequest ListFollowRequest
	if err := ctx.ShouldBindJSON(&followRequest); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ListLikesResponde]{
			Code: 4,
			Msg:  "绑定错误",
			Data: domain.ListLikesResponde{},
		})
		return
	}

	var cursor *domain.ListFollowCursor
	lastTime := followRequest.LatestTime
	idBefore := followRequest.IdBefore

	// 判断无效游标
	if idBefore < 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, RespModel.Respond[domain.ListLikesResponde]{
			Code: 4,
			Msg:  "idbefore必须大于0",
			Data: domain.ListLikesResponde{},
		})
		return
	} else {
		cursor = &domain.ListFollowCursor{
			TimeBefore: lastTime,
			IdBefore:   idBefore,
		}
	}

	followsResponde, err := fl.srv.ListFollows(ctx, author.UserId, followRequest.Limit, cursor)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, RespModel.Respond[domain.ListFollowResponde]{
			Code: 5,
			Msg:  "系统错误",
			Data: followsResponde,
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ListFollowResponde]{
		Code: 2,
		Msg:  "查看成功",
		Data: followsResponde,
	})
}
