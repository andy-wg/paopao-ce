package storage

import (
	"context"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/minio/minio-go/v7"
	"github.com/rocboss/paopao-ce/internal/core"
)

var (
	_ core.ObjectStorageService = (*minioServant)(nil)
	_ core.VersionInfo          = (*minioServant)(nil)
)

type minioServant struct {
	client             *minio.Client
	bucket             string
	domain             string
	retainInDays       time.Duration
	retainUntilDate    time.Time
	allowPersistObject bool
}

type s3Servant = minioServant

func (s *minioServant) Name() string {
	return "MinIO"
}

func (s *minioServant) Version() *semver.Version {
	return semver.MustParse("v0.2.0")
}

func (s *minioServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, persistance bool) (string, error) {
	opts := minio.PutObjectOptions{ContentType: contentType}
	if s.allowPersistObject && !persistance {
		opts.Mode = minio.Governance
		opts.RetainUntilDate = time.Now().Add(s.retainInDays)
	}
	_, err := s.client.PutObject(context.Background(), s.bucket, objectKey, reader, objectSize, opts)
	if err != nil {
		return "", err
	}
	return s.domain + objectKey, nil
}

func (s *minioServant) PersistObject(objectKey string) error {
	if !s.allowPersistObject {
		return nil
	}
	retentionMode := minio.Governance
	return s.client.PutObjectRetention(context.Background(), s.bucket, objectKey, minio.PutObjectRetentionOptions{
		Mode:            &retentionMode,
		RetainUntilDate: &s.retainUntilDate,
	})
}

func (s *minioServant) DeleteObject(objectKey string) error {
	return s.client.RemoveObject(context.Background(), s.bucket, objectKey, minio.RemoveObjectOptions{ForceDelete: true})
}

func (s *minioServant) DeleteObjects(objectKeys []string) (err error) {
	objectsCh := make(chan minio.ObjectInfo, len(objectKeys))

	resCh := s.client.RemoveObjects(context.Background(), s.bucket, objectsCh, minio.RemoveObjectsOptions{})
	for _, objectKey := range objectKeys {
		objectsCh <- minio.ObjectInfo{
			Key: objectKey,
		}
	}

	// 宽松处理所有错误，只记录最后一次发生的错误
	for result := range resCh {
		if result.Err != nil {
			err = result.Err
		}
	}
	return
}

func (s *minioServant) IsObjectExist(objectKey string) (bool, error) {
	_, err := s.client.StatObject(context.Background(), s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *minioServant) SignURL(objectKey string, expiredInSec int64) (string, error) {
	// TODO: Set request parameters for content-disposition.
	reqParams := make(url.Values)
	signedURL, err := s.client.PresignedGetObject(context.Background(), s.bucket, objectKey, time.Duration(expiredInSec)*time.Second, reqParams)
	if err != nil {
		return "", err
	}
	return signedURL.String(), nil
}

func (s *minioServant) ObjectURL(objetKey string) string {
	return s.domain + objetKey
}

func (s *minioServant) ObjectKey(objectUrl string) string {
	return strings.Replace(objectUrl, s.domain, "", -1)
}
