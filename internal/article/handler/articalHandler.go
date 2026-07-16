package web

import (
	"net/http"
	"webookApp/internal/article/domain"
	"webookApp/internal/article/service"
	"webookApp/internal/middelware/authentication"
	RespModel "webookApp/internal/respondeModel"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 使用TDD 完成接口开发
type ArticalHandler struct {
	srv *service.ArticalService
}

func NewArticalHander(srv *service.ArticalService) *ArticalHandler {
	return &ArticalHandler{
		srv: srv,
	}
}

func (ah *ArticalHandler) RegisterRoute(g *gin.Engine) {
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	// 这个是指在8MB以内的文件在内存中读取很快，超过了转存到系统的临时文件夹中,操作变成磁盘I/O
	g.MaxMultipartMemory = 8 << 20 // 8 MiB
	ag := g.Group("/article")
	ag.POST("/edit", ah.Edit)
	ag.POST("/publish", ah.Publish)
	ag.POST("/delete", ah.Delete)
	ag.POST("/upload", ah.Uploade)
	ag.POST("/auth/list", ah.List)
	ag.GET("/hide/:id", ah.Hide)
	ag.GET("/open/:id", ah.Open)
	ag.GET("/auth/check/:id", ah.CheckWithAuthor)
	ag.GET("/check/:id", ah.Check)

}

func (ah *ArticalHandler) Edit(ctx *gin.Context) {
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type Request struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
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
	// 校验类型是否正确先跳过
	err := ah.srv.Save(ctx, domain.Artical{
		Title:   req.Title,
		Content: req.Content,
		New:     true,
		Author: domain.Author{
			ID:   author.UserId,
			Name: author.Name,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 200,
		Msg:  "新建成功",
	})
}

func (ah *ArticalHandler) Publish(ctx *gin.Context) {
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type Request struct {
		Title   string               `json:"title" binding:"required"`
		Content string               `json:"content" binding:"required"`
		ArtID   int64                `json:"article_id"`
		Status  domain.ArticalStatus `json:"status"`
	}

	var req Request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}
	// 校验类型是否正确先跳过
	err := ah.srv.Publish(ctx, domain.Artical{
		Title:   req.Title,
		Content: req.Content,
		ArtiID:  req.ArtID,
		Author: domain.Author{
			ID:   author.UserId,
			Name: author.Name,
		},
		Status: req.Status,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 200,
		Msg:  "发表成功",
	})
}

func (ah *ArticalHandler) Hide(ctx *gin.Context) {
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
			Code: 4,
			Msg:  "令牌无效",
		})
		return
	}
	type artId struct {
		Id int64 `uri:"id" binding:"required"`
	}

	var artUri artId
	if err := ctx.ShouldBindUri(&artUri); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}
	err := ah.srv.Hide(ctx, domain.Artical{
		ArtiID: artUri.Id,
		Author: domain.Author{
			ID: author.UserId,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 200,
		Msg:  "隐藏成功",
	})
}

func (ah *ArticalHandler) Open(ctx *gin.Context) {
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
			Code: 4,
			Msg:  "令牌无效",
		})
		return
	}
	type artId struct {
		Id int64 `uri:"id" binding:"required"`
	}

	var artUri artId
	if err := ctx.ShouldBindUri(&artUri); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}
	err := ah.srv.Open(ctx, domain.Artical{
		ArtiID: artUri.Id,
		Author: domain.Author{
			ID: author.UserId,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 200,
		Msg:  "重新发表成功",
	})
}

func (ah *ArticalHandler) Delete(ctx *gin.Context) {
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
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌无效",
		})
		return
	}

	type Request struct {
		ArticalID int64 `json:"art_id" buinding:"required"`
	}

	var request Request
	if err := ctx.ShouldBind(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}
	err := ah.srv.Delete(ctx, domain.Artical{
		ArtiID: request.ArticalID,
		Author: domain.Author{
			ID: author.UserId,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[string]{
		Code: 200,
		Msg:  "删除帖子成功",
	})
}

// checkWithAuthor 作者查看自己的文章详情
func (ah *ArticalHandler) CheckWithAuthor(ctx *gin.Context) {
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

	type artId struct {
		Id int64 `uri:"id" binding:"required"`
	}

	var artUri artId
	if err := ctx.ShouldBindUri(&artUri); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	err, articel := ah.srv.CheckWithAuthor(ctx, artUri.Id, author.UserId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[domain.ArticleSearchRespond]{
			Code: 5,
			Msg:  "系统错误",
			Data: articel,
		})
		return
	}

	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ArticleSearchRespond]{
		Code: 2,
		Msg:  "查询成功",
		Data: articel,
	})
}

// check 读者文章详情
func (ah *ArticalHandler) Check(ctx *gin.Context) {
	type artId struct {
		Id int64 `uri:"id" binding:"required"`
	}

	var artUri artId
	if err := ctx.ShouldBindUri(&artUri); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	err, articel := ah.srv.Check(ctx, artUri.Id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[domain.ArticleSearchRespond]{
			Code: 5,
			Msg:  "系统错误",
			Data: articel,
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[domain.ArticleSearchRespond]{
		Code: 2,
		Msg:  "查询成功",
		Data: articel,
	})
}

func (ah *ArticalHandler) List(ctx *gin.Context) {
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}

	type request struct {
		Limit  int64  `json:"limit" binding:"required" `
		Offset *int64 `json:"offset" binding:"required"`
	}

	var req request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, RespModel.Respond[string]{
			Code: 5,
			Msg:  "绑定错误",
			Data: err.Error(),
		})
		return
	}

	err, content := ah.srv.List(ctx, author.UserId, req.Limit, *req.Offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[[]domain.ArticleSearchRespond]{
			Code: 5,
			Msg:  "系统错误",
			Data: content,
		})
		return
	}
	ctx.JSON(http.StatusOK, RespModel.Respond[[]domain.ArticleSearchRespond]{
		Code: 2,
		Msg:  "查询成功",
		Data: content,
	})
}

// 如果文章有图片，在编辑问题前可以先上传图片使用OSS服务
// 因为编写文章分成了两步：
// 1.先上传图片
// 2.后撰写文字部分
// 暂时想到的问题：1.上传图片成功后携带minio生成的URL怎么重定向到编写文章的地方？
// 2. 上传图片失败后接入监控，重试，兜底方案
// 2.1 oss服务器真的崩了，还要不要继续撰写文字部分??
// 3. 如果文字部分崩了比如数据库崩了,怎么回滚在minio存储的图片？
// 3.1 文字部分崩了，接入监控，重试，兜底方案,怎么恢复数据??
// 4. 谁来承担为文字部分和图片部分传递消息的中间人??
func (ah *ArticalHandler) Uploade(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 10<<20) // 限制传文件大小
	auth_, exists := ctx.Get("accessToken")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌不存在",
			Data: 0,
		})
		return
	}
	author, ok := auth_.(*authentication.UserClaim)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, RespModel.Respond[int64]{
			Code: 4,
			Msg:  "令牌无效",
			Data: 0,
		})
		return
	}
	form, err := ctx.MultipartForm()
	// 释放超过内存读取文件限制而产生的临时文件
	defer ctx.Request.MultipartForm.RemoveAll()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 501,
			Msg:  "未知错误",
			Data: err.Error(),
		})
		zap.L().Error("解析表单错误:", zap.Error(err))
		return
	}
	tmpUrls, err := ah.srv.OssUpload(ctx, form.File["files"], author.UserId)
	if err != nil {
		// 2. 上传图片失败后接入监控，重试，兜底方案
		ctx.JSON(http.StatusInternalServerError, RespModel.Respond[string]{
			Code: 501,
			Msg:  "上传图片出错了",
			Data: err.Error(),
		})
		zap.L().Error("upload 接口错误:", zap.Error(err))
		return
	}
	// 2. 上传图片失败后接入监控，重试，兜底方案
	ctx.JSON(http.StatusOK, RespModel.Respond[[]string]{
		Code: 200,
		Msg:  "上传图片成功",
		Data: tmpUrls,
	})
}
