package oss

import (
	"time"
)

type PutObjectOptions struct {
	UserMetadata       map[string]string // Map of user metadata
	UserTags           map[string]string // Map of user object tags
	ContentType        string            // Content type of object, e.g “application/text”
	ContentEncoding    string            // Content encoding of object, e.g “gzip”
	CacheControl       string            // Used to specify directives for caching mechanisms in both requests and responses e.g “max-age=600”
	Expires            time.Time         // expiretime of object
	ContentDisposition string            // make download or review url
}

type BucketName string

const (
	DraftAndPublishBucketName BucketName = "webook"
)
