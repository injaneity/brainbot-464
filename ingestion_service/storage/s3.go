package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	prefix        string
}

func NewS3Client(ctx context.Context, bucket, prefix, region string) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(client)

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	return &S3Client{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
		prefix:        prefix,
	}, nil
}

func (s *S3Client) CreateArticleObject(ctx context.Context, id, title, content string) error {
	key := s.prefix + id

	// Format: Title\nContent
	data := fmt.Sprintf("%s\n%s", title, content)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object to S3: %w", err)
	}
	return nil
}

func (s *S3Client) AppendToArticleObject(ctx context.Context, id, newContent string) error {
	key := s.prefix + id

	// 1. Get existing object
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer resp.Body.Close()

	existingData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read object body: %w", err)
	}

	// 2. Append new content with separator
	// "we use \n--\n to indicate separate article content"
	newData := string(existingData) + "\n--\n" + newContent

	// 3. Upload updated object
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(newData),
	})
	if err != nil {
		return fmt.Errorf("failed to update object in S3: %w", err)
	}

	log.Printf("Appended content to S3 object %s", key)
	return nil
}

func (s *S3Client) GeneratePresignedURL(ctx context.Context, id string, lifetime time.Duration) (string, error) {
	key := s.prefix + id

	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = lifetime
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}
