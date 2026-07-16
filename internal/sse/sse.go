package sse

import (
	"fmt"
	"net/http"
	"sync"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"

	"github.com/gin-gonic/gin"
)

// 管理所有在线用户SSE连接通道
type SSEManagerHandler struct {
	mu      sync.RWMutex
	clients map[int32]chan string // 用户ID->消息通道
}

func NewSSEManagerHandler() *SSEManagerHandler {
	return &SSEManagerHandler{
		clients: make(map[int32]chan string),
	}
}

// 注册一个用户的连接，返回一个通道，业务方通过此通道推送消息
func (sh *SSEManagerHandler) Register(userID int32) chan string {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	ch := make(chan string, 200)
	sh.clients[userID] = ch
	return ch
}

// 用户断开连接时注销
func (sh *SSEManagerHandler) UnRegister(userID int32) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if ch, ok := sh.clients[userID]; ok {
		close(ch)
		delete(sh.clients, userID)
	}
}

// 向指定用户推送消息
func (sh *SSEManagerHandler) SentToUser(userID int32, message string) {
	sh.mu.RLocker().Lock()
	defer sh.mu.RLocker().Unlock()
	if ch, ok := sh.clients[userID]; ok {
		// 非阻塞发送,通道满了卡住
		select {
		case ch <- message:
		default:
			// 通道满了直接丢弃
		}
	}
}

func (sh *SSEManagerHandler) RegisterRoute(g *gin.Engine) {
	ag := g.Group("/notify")
	ag.GET("/connect", sh.Notify) // 作者自己通知页面地址
}

func (sh *SSEManagerHandler) Notify(ctx *gin.Context) {
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
	// 1. 设置SSE必须的响应头
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	//2.注册自己的专属用户ID->消息通道
	msgChan := sh.Register(author.UserId)
	defer sh.UnRegister(author.UserId)

	// 3.拿到flush,缓冲区数据立刻发送到客户端
	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		ctx.String(500, "不支持流")
		return
	}

	// 3.监听这个通道
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				// 通道关闭
				return
			}
			fmt.Fprintf(ctx.Writer, "Notify:%s", msg)
			flusher.Flush()
		case <-ctx.Request.Context().Done():
			// 客户端断开连接
			return
		}
	}
}
