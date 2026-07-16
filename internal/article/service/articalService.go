package service

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"time"
	"webookApp/internal/article/domain"
	"webookApp/internal/article/repository"
	"webookApp/pkg/oss"
	"webookApp/pkg/snowflake"
)

type ArticalService struct {
	aRepository *repository.ArticalRepository
	ossServer   *oss.OssServer
	sf          snowflake.SnowFlaker
}

func NewArticalService(articalRepo *repository.ArticalRepository, ossServer *oss.OssServer, sf snowflake.SnowFlaker) *ArticalService {
	return &ArticalService{
		aRepository: articalRepo,
		ossServer:   ossServer,
		sf:          sf,
	}
}

func (arc *ArticalService) Save(ctx context.Context, ad domain.Artical) error {
	id := arc.sf.GenerateID()
	timeStamp := time.Now().UTC().UnixMilli()
	ad.ArtiID = id
	ad.Ctime = timeStamp
	ad.Utime = timeStamp
	return arc.aRepository.Add(ctx, ad)
}

func (arc *ArticalService) Publish(ctx context.Context, ad domain.Artical) error {
	timeStamp := time.Now().UTC().UnixMilli()
	if ad.ArtiID == 0 {
		id := arc.sf.GenerateID()
		ad.ArtiID = id
		ad.Ctime = timeStamp
		ad.Utime = timeStamp
		ad.New = true
	} else {
		ad.Utime = timeStamp
		ad.New = false
	}
	// 发表成功，放入粉丝收件箱
	publishErr := arc.aRepository.Publish(ctx, ad, domain.ArticalPublished)
	if publishErr != nil {
		return publishErr
	}
	// 查找粉丝id
	findFansErr, idList := arc.aRepository.SearchFanList(ctx, ad.Author.ID)
	switch findFansErr {
	case sql.ErrNoRows:
		return nil
	case nil: // 拿到粉丝id列表
		return arc.aRepository.InsertBox(ctx, idList, ad.ArtiID, ad.Utime)
	default:
		return nil
	}
}

func (arc *ArticalService) Hide(ctx context.Context, ad domain.Artical) error {
	timeStamp := time.Now().UTC().UnixMilli()
	ad.Utime = timeStamp
	return arc.aRepository.UpdateStatus(ctx, ad, domain.ArticalHidend)
}

func (arc *ArticalService) Open(ctx context.Context, ad domain.Artical) error {
	timeStamp := time.Now().UTC().UnixMilli()
	ad.Utime = timeStamp
	return arc.aRepository.UpdateStatus(ctx, ad, domain.ArticalPublished)
}

func (arc *ArticalService) Delete(ctx context.Context, ad domain.Artical) error {
	timeStamp := time.Now().UTC().UnixMilli()
	ad.Utime = timeStamp
	return arc.aRepository.UpdateStatus(ctx, ad, domain.ArticalDeleted)
}

func (arc *ArticalService) Check(ctx context.Context, articleID int64) (error, domain.ArticleSearchRespond) {
	return arc.aRepository.Check(ctx, articleID)
}

func (arc *ArticalService) CheckWithAuthor(ctx context.Context, articleID int64, userId int32) (error, domain.ArticleSearchRespond) {
	return arc.aRepository.CheckWithAuthor(ctx, articleID, userId)
}

func (arc *ArticalService) List(ctx context.Context, userID int32, limit, offset int64) (error, []domain.ArticleSearchRespond) {
	return arc.aRepository.List(ctx, userID, limit, offset)
}

func (arc *ArticalService) OssUpload(ctx context.Context, files []*multipart.FileHeader, authID int32) ([]string, error) {
	var urls []string
	for _, file := range files {
		fd, err := file.Open()
		if err != nil {
			return urls, fmt.Errorf("file open err:%s", err.Error())
		}

		url, err := arc.ossServer.TempUrl(ctx, string(oss.DraftAndPublishBucketName), fmt.Sprintf("tmp/%v/%s", time.Now().UTC().Year(), file.Filename), fd, file.Size, oss.PutObjectOptions{
			UserMetadata: map[string]string{
				"user":        string(authID),
				"upload_time": time.Now().UTC().Format("2006-01-02 00:00:00"),
			},
			ContentType:        "image/png", // 只有png才能生成预览链接而不是下载链接
			ContentDisposition: "inline",
		})

		fd.Close()
		if err != nil {
			return urls, fmt.Errorf("osssServer err:%s", err.Error())
		}
		urls = append(urls, url)
	}

	return urls, nil
}
