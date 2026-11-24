// Package storage wraps S3-compatible object storage (R2, S3, MinIO).
package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mbhat24/ugc-ad-generator/backend/internal/config"
)

type Store struct {
	c       *s3.Client
	presign *s3.PresignClient
	bucket  string
	private bool
	public  string
}

func New(ctx context.Context, cfg config.Config) (*Store, error) {
	if !cfg.StorageEnabled() {
		return nil, nil
	}
	ac, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")))
	if err != nil {
		return nil, err
	}
	c := s3.NewFromConfig(ac, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3ForcePathStyle
	})
	return &Store{c: c, presign: s3.NewPresignClient(c), bucket: cfg.S3Bucket, private: cfg.S3Private, public: cfg.S3PublicURL}, nil
}

// Private reports whether objects are served through signed URLs.
func (s *Store) Private() bool { return s.private }

// Upload stores a local MP4. It returns (key, "") for private storage or ("", publicURL) otherwise.
func (s *Store) Upload(ctx context.Context, path, key string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	_, err = s.c.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: f, ContentType: aws.String("video/mp4"), CacheControl: aws.String("private, max-age=31536000, immutable")})
	if err != nil {
		return "", "", fmt.Errorf("upload: %w", err)
	}
	if s.private {
		return key, "", nil
	}
	return "", s.public + "/" + key, nil
}

func (s *Store) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	r, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key, ResponseContentDisposition: aws.String(`attachment; filename="adreel-video.mp4"`)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return r.URL, nil
}

func (s *Store) DeletePrefix(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("prefix must end with /")
	}
	p := s3.NewListObjectsV2Paginator(s.c, &s3.ListObjectsV2Input{Bucket: &s.bucket, Prefix: &prefix})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]types.ObjectIdentifier, len(page.Contents))
		for i, o := range page.Contents {
			ids[i] = types.ObjectIdentifier{Key: o.Key}
		}
		if _, err := s.c.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: &s.bucket, Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)}}); err != nil {
			return err
		}
	}
	return nil
}
