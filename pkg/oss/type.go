package oss

import (
	"context"
	"io"
)

type OssServicer interface {
	MakeBucket(ctx context.Context, bucketName string) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts PutObjectOptions) (string, error)
}
