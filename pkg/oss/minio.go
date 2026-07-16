package oss

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type OssServer struct {
	client        *minio.Client
	region        string // Bucket location
	objectLocking bool   // Enable object locking
	forceCreate   bool   // ForceCreate - this is a MinIO specific extension.

}

// 为了在执行wire的时候相同类型的参数注入不会报错
type CreateOssServerConf struct {
	Endpoint      string
	AccessKeyID   string
	SecreteKeyID  string
	Region        string
	token         string
	UseSecure     bool
	ObjectLocking bool
	ForceCreate   bool
}

func NewOssServer(opts CreateOssServerConf) *OssServer {
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKeyID, opts.SecreteKeyID, opts.token),
		Secure: opts.UseSecure,
	})
	if err != nil {
		zap.L().Fatal("ossFail:", zap.Error(err))
	}
	return &OssServer{
		client:        client,
		region:        opts.Region,
		objectLocking: opts.ObjectLocking,
		forceCreate:   opts.ForceCreate,
	}
}

// 创建bucket
func (oss *OssServer) MakeBucket(ctx context.Context, bucketName string) error {
	err := oss.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region:        oss.region,
		ObjectLocking: oss.objectLocking,
		ForceCreate:   oss.forceCreate,
	})
	if err != nil {
		exists, errBucketexist := oss.client.BucketExists(ctx, bucketName)
		if errBucketexist == nil && exists {
			return nil
		} else {
			return err
		}
	}
	return err
}

// PutObject 上传帖子的图片/视频等
// 获得浏览的图片/视频miniO URL地址 保存到数据库里
func (oss *OssServer) TempUrl(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts PutObjectOptions) (string, error) {
	_, err := oss.client.PutObject(ctx, bucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		UserMetadata:       opts.UserMetadata,
		UserTags:           opts.UserTags,
		ContentType:        opts.ContentType,
		ContentEncoding:    opts.ContentEncoding,
		CacheControl:       opts.CacheControl,
		Expires:            opts.Expires,
		ContentDisposition: opts.ContentDisposition,
	})
	if err != nil {
		return "", err
	}
	addr := fmt.Sprintf("preview=true&prefix=%s&version_id=null", objectName)
	return addr, err
}

func (oss *OssServer) CopyObject(ctx context.Context, sourceBuckt, desBuckt, sourceobjectname, objectname string) error {
	srcOpts := minio.CopySrcOptions{
		Bucket: sourceBuckt,
		Object: sourceobjectname,
	}

	// Destination object
	dstOpts := minio.CopyDestOptions{
		Bucket: desBuckt,
		Object: objectname,
	}
	_, err := oss.client.CopyObject(ctx, dstOpts, srcOpts)
	if err != nil {
		return fmt.Errorf("复制tmp的文件失败:%s", err)
	}
	return err
}
