package utils

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ObjectInfo contains metadata for an S3 object.
type ObjectInfo struct {
	Key          string
	LastModified time.Time
	Size         int64
}

func derefInt64(v *int64) int64 {
	if v == nil { return 0 }
	return *v
}

const (
	DefaultBucket = "tazlab-storage"
	DefaultRegion = "eu-central-1"
)

// S3Client handles communication with AWS S3
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client initializes a new S3 client with optional SSO profile support.
// bucket and region may be empty to use the built-in defaults.
func NewS3Client(bucket, region, profile string) (*S3Client, error) {
	if bucket == "" {
		bucket = DefaultBucket
	}
	if region == "" {
		region = DefaultRegion
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
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

// HeadObject retrieves metadata for an object without downloading it.
func (s *S3Client) HeadObject(key string) (map[string]string, error) {
	result, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head object %s: %w", key, err)
	}
	return result.Metadata, nil
}

// UploadFileWithMetadata uploads a file with custom metadata.
func (s *S3Client) UploadFileWithMetadata(key, filePath string, metadata map[string]string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file %v, %v", filePath, err)
	}
	defer file.Close()

	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		Body:     file,
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("unable to upload file %v to bucket %v, %v", filePath, s.bucket, err)
	}
	return nil
}

// ListObjects lists objects under the given prefix, handling pagination.
func (s *S3Client) ListObjects(prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("list objects %s: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil || obj.LastModified == nil {
				continue
			}
			objects = append(objects, ObjectInfo{
				Key:          *obj.Key,
				LastModified: *obj.LastModified,
				Size:         derefInt64(obj.Size),
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(objects[j].LastModified)
	})
	return objects, nil
}

// DeleteObjects deletes multiple objects from the bucket.
func (s *S3Client) DeleteObjects(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	identifiers := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		identifiers[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	_, err := s.client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &types.Delete{
			Objects: identifiers,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("delete objects: %w", err)
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

