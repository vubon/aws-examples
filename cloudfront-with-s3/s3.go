package main

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type IS3 interface {
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	PutBucketPolicy(ctx context.Context, params *s3.PutBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.PutBucketPolicyOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type IS3PreSigned interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type S3Client struct {
	Svc    IS3
	PreSvc IS3PreSigned
	Region string
}

func New() (*S3Client, error) {
	env, ok := os.LookupEnv("ENV")
	var cfg aws.Config
	var err error

	if ok && env == "local" {
		awsRegion, _ := os.LookupEnv("AWS_REGION")
		awsProfile, _ := os.LookupEnv("AWS_PROFILE")
		awsEndpoint, _ := os.LookupEnv("AWS_ENDPOINT")
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(awsRegion),
			config.WithSharedConfigProfile(awsProfile),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						PartitionID:   "aws",
						URL:           awsEndpoint,
						SigningRegion: awsRegion,
					}, nil
				})))
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background())
	}

	if err != nil {
		return nil, err
	}
	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Client{Svc: svc, PreSvc: s3.NewPresignClient(svc), Region: cfg.Region}, nil
}

func (c *S3Client) CreateBucket(name string) string {
	bucket, err := c.Svc.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(name),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.Region),
		},
	})
	if err != nil {
		log.Println("bucket create error ", err)
		return ""
	}

	return *bucket.Location
}

func (c *S3Client) PolicyUpdate(name, policy string) error {
	_, err := c.Svc.PutBucketPolicy(context.Background(), &s3.PutBucketPolicyInput{
		Bucket: aws.String(name),
		Policy: aws.String(policy),
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *S3Client) Upload(key, bucket string, body io.Reader) bool {
	_, err := c.Svc.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		log.Println("File cloudfront-with-s3 error Key: ", key)
		return false
	}
	return true
}

func (c *S3Client) GetFileList(bucket string) []types.Object {
	result, err := c.Svc.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		log.Println("Get Object list error ", err)
		return nil
	}
	return result.Contents
}

func (c *S3Client) PresignGetObject(key, bucket string) (*v4.PresignedHTTPRequest, error) {
	object, err := c.PreSvc.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opt *s3.PresignOptions) {
		opt.Expires = 168 * time.Hour // expire within 7 days
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func (c *S3Client) GeneratePreSignedURL(key, bucket string, body io.Reader) string {
	ok := c.Upload(key, bucket, body)
	if ok {
		object, err := c.PresignGetObject(key, bucket)
		if err != nil {
			log.Println("PreSign URL generation error ", err)
		}
		return object.URL
	}
	return ""
}
