package common

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config contains minimal configuration for creating an S3 client.
// Values are optional and will fall back to the standard AWS config/credential chain.
type S3Config struct {
	// Region to use for requests, e.g. "us-east-1". If empty, AWS defaults apply.
	Region string
	// Profile selects a named shared config/credentials profile. If empty, default chain applies.
	Profile string
	// UsePathStyle forces path-style addressing (useful for some S3-compatible providers).
	UsePathStyle bool
}

// S3 wraps the AWS SDK for Go v2 S3 client with a narrow interface we can mock.
type S3 struct {
	client *s3.Client
}

// NewS3 creates a new S3 wrapper using the default AWS configuration chain,
// with optional overrides from S3Config.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	var loadOpts []func(*config.LoadOptions) error
	if cfg.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(cfg.Region))
	}
	if cfg.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	c := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})
	return &S3{client: c}, nil
}

// Put uploads an object to the given bucket/key.
// If contentType is non-empty, it is set on the object.
func (s *S3) Put(ctx context.Context, bucket, key string, body io.Reader, contentType string, cacheControl string, acl s3types.ObjectCannedACL) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if cacheControl != "" {
		in.CacheControl = aws.String(cacheControl)
	}
	if acl != "" {
		in.ACL = acl
	}

	_, err := s.client.PutObject(ctx, in)
	return err
}

// Get fetches an object and returns its streaming body. Caller must Close it.
func (s *S3) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

// Head retrieves the object's metadata without returning the body.
func (s *S3) Head(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	return s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// Delete removes the object at bucket/key.
func (s *S3) Delete(ctx context.Context, bucket, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// Exists returns true if the object exists (HTTP 200 from HeadObject); false if 404/NotFound.
func (s *S3) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := s.Head(ctx, bucket, key)
	if err == nil {
		return true, nil
	}

	// Check for HTTP 404 response error
	var respErr *http.ResponseError
	if errors.As(err, &respErr) {
		if respErr.HTTPStatusCode() == 404 {
			return false, nil
		}
	}

	// Check for API error code NotFound
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "NotFound" {
			return false, nil
		}
	}

	return false, err
}

// List lists objects with the given prefix. Use continuationToken for pagination.
func (s *S3) List(ctx context.Context, bucket, prefix string, maxKeys int32, continuationToken *string) (*s3.ListObjectsV2Output, error) {
	return s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucket),
		Prefix:            aws.String(prefix),
		MaxKeys:           aws.Int32(maxKeys),
		ContinuationToken: continuationToken,
	})
}

// Client exposes the underlying SDK client for advanced callers (avoid when possible).
func (s *S3) Client() *s3.Client { return s.client }
