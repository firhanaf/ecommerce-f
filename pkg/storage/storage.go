package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ─── Interface ───────────────────────────────────────────────────────────────
// Mau ganti ke GCS atau MinIO? Buat struct baru yang implement interface ini.
// Usecase tidak perlu tahu implementasinya.

type Storage interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
	Delete(ctx context.Context, key string) error
	GetURL(key string) string
}

// ─── S3 Implementation ───────────────────────────────────────────────────────

type s3Storage struct {
	client *s3.Client
	bucket string
	region string
}

func NewS3Storage(ctx context.Context, bucket, region string) (Storage, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &s3Storage{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		region: region,
	}, nil
}

func (s *s3Storage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s.GetURL(key), nil
}

func (s *s3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *s3Storage) GetURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
}

// ─── Helper ──────────────────────────────────────────────────────────────────

// GenerateKey membuat path unik untuk file di S3
// contoh: products/2024/01/abc123.jpg
func GenerateKey(folder, originalFilename string) string {
	ext := filepath.Ext(originalFilename)
	now := time.Now()
	return fmt.Sprintf("%s/%d/%02d/%s%s",
		folder,
		now.Year(),
		now.Month(),
		uuid.New().String(),
		ext,
	)
}

// ContentTypeFromFilename deteksi content type dari ekstensi file
func ContentTypeFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}
