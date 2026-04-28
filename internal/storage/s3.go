package storage

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client wraps the minio client for Forge artifacts.
type S3Client struct {
	client *minio.Client
	bucket string
}

// NewS3Client creates a new MinIO/S3 client and ensures the target bucket exists.
func NewS3Client(endpoint, accessKey, secretKey, bucket string) (*S3Client, error) {
	// Simple heuristic: if endpoint doesn't contain a schema, we assume no SSL
	// unless it's a known AWS endpoint.
	useSSL := strings.Contains(endpoint, "https://") || strings.Contains(endpoint, "amazonaws.com")
	
	// Clean schema from endpoint as minio expects host:port
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init s3 client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure bucket exists
	exists, err := cli.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
	}

	return &S3Client{
		client: cli,
		bucket: bucket,
	}, nil
}

// UploadArtifact uploads a file to S3 and returns the uploaded object key and size.
// The key format is: builds/{buildID}/jobs/{jobID}/{filename}
func (s *S3Client) UploadArtifact(ctx context.Context, buildID, jobID uint, filePath string) (string, int64, error) {
	filename := filepath.Base(filePath)
	objectKey := fmt.Sprintf("builds/%d/jobs/%d/%s", buildID, jobID, filename)

	info, err := s.client.FPutObject(ctx, s.bucket, objectKey, filePath, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", 0, fmt.Errorf("upload artifact: %w", err)
	}

	return objectKey, info.Size, nil
}

// GeneratePresignedURL creates a temporary download URL valid for 1 hour.
func (s *S3Client) GeneratePresignedURL(ctx context.Context, objectKey string) (string, error) {
	// Set content-disposition to attachment so the browser downloads it
	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(objectKey)))

	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, time.Hour, reqParams)
	if err != nil {
		return "", fmt.Errorf("presigned url: %w", err)
	}

	// Hack for local docker-compose testing: rewrite internal hostname to localhost
	if presignedURL.Host == "minio:9000" {
		presignedURL.Host = "localhost:9000"
	}

	return presignedURL.String(), nil
}
