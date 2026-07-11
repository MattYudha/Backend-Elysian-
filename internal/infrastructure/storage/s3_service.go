package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Service struct {
	client *minio.Client
	bucket string
	isMock bool
}

func NewS3Service(cfg *config.StorageConfig) (*S3Service, error) {
	if cfg.Endpoint == "" {
		// Return mock service for local offline testing
		return &S3Service{client: nil, bucket: "mock-bucket", isMock: true}, nil
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	return &S3Service{client: client, bucket: cfg.Bucket, isMock: false}, nil
}

// PresignPutURL generates a short-lived presigned URL for direct upload from browser.
func (s *S3Service) PresignPutURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if s.isMock {
		return fmt.Sprintf("http://localhost:7777/mock-upload?key=%s", objectKey), nil
	}
	presignedURL, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return presignedURL.String(), nil
}

// GetObjectURL returns a presigned GET URL for temporary access.
func (s *S3Service) GetObjectURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if s.isMock {
		return fmt.Sprintf("http://localhost:7777/api/v1/documents/mock-download?key=%s", objectKey), nil
	}
	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}
	return presignedURL.String(), nil
}

// DownloadToTemp downloads the S3 object to a local temp file and returns its path.
func (s *S3Service) DownloadToTemp(ctx context.Context, objectKey string) (string, error) {
	if s.isMock {
		mockFilePath := filepath.Join("tmp", "s3_mock", objectKey)
		tempPath := fmt.Sprintf("tmp/elysian_%d_%s", time.Now().UnixNano(), filepath.Base(objectKey))
		
		err := os.MkdirAll(filepath.Dir(tempPath), 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir: %w", err)
		}

		srcFile, err := os.Open(mockFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to open mock S3 file: %w", err)
		}
		defer srcFile.Close()

		destFile, err := os.Create(tempPath)
		if err != nil {
			return "", fmt.Errorf("failed to create temp download file: %w", err)
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return "", fmt.Errorf("failed to copy mock S3 file: %w", err)
		}

		return tempPath, nil
	}

	tempPath := fmt.Sprintf("/tmp/elysian_%d_%s", time.Now().UnixNano(), objectKey)
	err := s.client.FGetObject(ctx, s.bucket, objectKey, tempPath, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to download S3 object: %w", err)
	}
	return tempPath, nil
}

// EnsureBucket creates the bucket if it does not exist.
func (s *S3Service) EnsureBucket(ctx context.Context) error {
	if s.isMock {
		return nil
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket '%s': %w", s.bucket, err)
		}
	}
	return nil
}
