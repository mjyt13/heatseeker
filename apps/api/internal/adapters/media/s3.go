package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
	"heatseeker/api/internal/platform/filenames"
)

// S3 stores objects in an S3-compatible bucket (MinIO in dev and prod).
type S3 struct {
	client *minio.Client // server-side operations
	public *minio.Client // presigning with the client-facing origin
	bucket string
}

// NewS3 connects and makes sure the bucket exists.
func NewS3(ctx context.Context, cfg config.Storage) (*S3, error) {
	client, err := newMinio(cfg.S3Endpoint, cfg)
	if err != nil {
		return nil, err
	}
	public := client
	if cfg.S3PublicEndpoint != "" && cfg.S3PublicEndpoint != cfg.S3Endpoint {
		if public, err = newMinio(cfg.S3PublicEndpoint, cfg); err != nil {
			return nil, err
		}
	}
	s := &S3{client: client, public: public, bucket: cfg.S3Bucket}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(checkCtx, cfg.S3Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3: check bucket %q: %w", cfg.S3Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(checkCtx, cfg.S3Bucket, minio.MakeBucketOptions{Region: cfg.S3Region}); err != nil {
			return nil, fmt.Errorf("s3: create bucket %q: %w", cfg.S3Bucket, err)
		}
	}
	return s, nil
}

func newMinio(endpoint string, cfg config.Storage) (*minio.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("s3: bad endpoint %q", endpoint)
	}
	lookup := minio.BucketLookupAuto
	if cfg.S3ForcePathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3AccessKeyID, cfg.S3SecretKey, ""),
		Secure:       u.Scheme == "https",
		Region:       cfg.S3Region, // fixed region: presigning needs no network round trip
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: client: %w", err)
	}
	return client, nil
}

// Kind implements domain.MediaStore.
func (s *S3) Kind() domain.StorageKind { return domain.StorageS3 }

// PresignPut implements domain.MediaStore.
func (s *S3) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (*domain.PresignedRequest, error) {
	u, err := s.public.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return nil, fmt.Errorf("s3 presign put: %w", err)
	}
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return &domain.PresignedRequest{Method: http.MethodPut, URL: u.String(), Headers: headers, ExpiresAt: time.Now().Add(ttl).UTC()}, nil
}

// PresignGet implements domain.MediaStore.
func (s *S3) PresignGet(ctx context.Context, key, fileName string, inline bool, ttl time.Duration) (*domain.PresignedRequest, error) {
	params := url.Values{}
	params.Set("response-content-disposition", filenames.ContentDisposition(fileName, inline))
	u, err := s.public.PresignedGetObject(ctx, s.bucket, key, ttl, params)
	if err != nil {
		return nil, fmt.Errorf("s3 presign get: %w", err)
	}
	return &domain.PresignedRequest{Method: http.MethodGet, URL: u.String(), ExpiresAt: time.Now().Add(ttl).UTC()}, nil
}

// Put implements domain.MediaStore. size may be -1 (multipart).
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if _, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

// Open implements domain.MediaStore.
func (s *S3) Open(ctx context.Context, key string, offset, length int64) (io.ReadCloser, *domain.ObjectInfo, error) {
	opts := minio.GetObjectOptions{}
	switch {
	case length > 0:
		if err := opts.SetRange(offset, offset+length-1); err != nil {
			return nil, nil, fmt.Errorf("s3 open: %w", err)
		}
	case offset > 0:
		if err := opts.SetRange(offset, 0); err != nil {
			return nil, nil, fmt.Errorf("s3 open: %w", err)
		}
	}
	info, err := s.Stat(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if length == 0 {
		return io.NopCloser(strings.NewReader("")), info, nil
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, nil, mapS3Err(err)
	}
	return obj, info, nil
}

// Stat implements domain.MediaStore.
func (s *S3) Stat(ctx context.Context, key string) (*domain.ObjectInfo, error) {
	st, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, mapS3Err(err)
	}
	return &domain.ObjectInfo{Key: key, Size: st.Size, ContentType: st.ContentType, ModTime: st.LastModified}, nil
}

// Move implements domain.MediaStore (server-side copy + delete).
func (s *S3) Move(ctx context.Context, src, dst string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dst},
		minio.CopySrcOptions{Bucket: s.bucket, Object: src},
	)
	if err != nil {
		return mapS3Err(err)
	}
	return s.Delete(ctx, src)
}

// Delete implements domain.MediaStore.
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return mapS3Err(err)
	}
	return nil
}

func mapS3Err(err error) error {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) && (resp.Code == "NoSuchKey" || resp.StatusCode == http.StatusNotFound) {
		return domain.NotFound("object")
	}
	return fmt.Errorf("s3: %w", err)
}
