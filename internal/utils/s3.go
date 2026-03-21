package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	DefaultBucket = "tazlab-storage"
	DefaultRegion = "eu-central-1"
)

// S3Client handles communication with AWS S3
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client initializes a new S3 client with optional SSO profile support
func NewS3Client(bucket, profile string) (*S3Client, error) {
	if bucket == "" {
		bucket = DefaultBucket
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(DefaultRegion),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %v", err)
	}

	return &S3Client{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

// UploadFile uploads a file to S3
func (s *S3Client) UploadFile(key, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file %v, %v", filePath, err)
	}
	defer file.Close()

	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("unable to upload file %v to bucket %v, %v", filePath, s.bucket, err)
	}

	return nil
}

// DownloadFile downloads a file from S3
func (s *S3Client) DownloadFile(key, filePath string) error {
	result, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("unable to download item %v from bucket %v, %v", key, s.bucket, err)
	}
	defer result.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("unable to create file %v, %v", filePath, err)
	}
	defer file.Close()

	_, err = file.ReadFrom(result.Body)
	if err != nil {
		return fmt.Errorf("unable to save file %v, %v", filePath, err)
	}

	return nil
}

// CreateBucket creates the target bucket if it doesn't exist
func (s *S3Client) CreateBucket() error {
	fmt.Printf("☁️  Creating bucket %s in %s...\n", s.bucket, DefaultRegion)
	_, err := s.client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraintEuCentral1,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %v", err)
	}
	return nil
}

