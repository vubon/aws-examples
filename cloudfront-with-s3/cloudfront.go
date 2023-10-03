package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/smithy-go/rand"
)

type ICloudFront interface {
	CreateKeyGroup(ctx context.Context, input *cloudfront.CreateKeyGroupInput, optFns ...func(*cloudfront.Options)) (*cloudfront.CreateKeyGroupOutput, error)
	CreatePublicKey(ctx context.Context, params *cloudfront.CreatePublicKeyInput, optFns ...func(*cloudfront.Options)) (*cloudfront.CreatePublicKeyOutput, error)
	CreateOriginAccessControl(ctx context.Context, params *cloudfront.CreateOriginAccessControlInput, optFns ...func(*cloudfront.Options)) (*cloudfront.CreateOriginAccessControlOutput, error)
	CreateDistribution(ctx context.Context, params *cloudfront.CreateDistributionInput, optFns ...func(*cloudfront.Options)) (*cloudfront.CreateDistributionOutput, error)
	GetPublicKey(ctx context.Context, params *cloudfront.GetPublicKeyInput, optFns ...func(*cloudfront.Options)) (*cloudfront.GetPublicKeyOutput, error)
	GetKeyGroup(ctx context.Context, params *cloudfront.GetKeyGroupInput, optFns ...func(*cloudfront.Options)) (*cloudfront.GetKeyGroupOutput, error)
}

type CFClient struct {
	Svc ICloudFront
}

func NewCFClient() (*CFClient, error) {
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
	svc := cloudfront.NewFromConfig(cfg)

	return &CFClient{svc}, nil
}

func (c *CFClient) CreateKeyGroup(name, comment string, items []string) (string, error) {
	group, err := c.Svc.CreateKeyGroup(context.Background(), &cloudfront.CreateKeyGroupInput{KeyGroupConfig: &types.KeyGroupConfig{
		Items:   items,
		Name:    aws.String(name),
		Comment: aws.String(comment),
	}})
	if err != nil {
		return "", err
	}
	return *group.KeyGroup.Id, nil
}

func (c *CFClient) CreatePublicKey(name, comment, publicKey string) (string, error) {
	uuid, _ := rand.NewUUID(rand.Reader).GetUUID()
	key, err := c.Svc.CreatePublicKey(context.Background(), &cloudfront.CreatePublicKeyInput{PublicKeyConfig: &types.PublicKeyConfig{
		CallerReference: aws.String(uuid),
		EncodedKey:      aws.String(publicKey),
		Name:            aws.String(name),
		Comment:         aws.String(comment),
	}})
	if err != nil {
		return "", err
	}
	return *key.PublicKey.Id, nil
}

func (c *CFClient) CreateOriginAccessControl(name string) (string, error) {
	created, err := c.Svc.CreateOriginAccessControl(context.Background(), &cloudfront.CreateOriginAccessControlInput{OriginAccessControlConfig: &types.OriginAccessControlConfig{
		Name:                          aws.String(name),
		OriginAccessControlOriginType: types.OriginAccessControlOriginTypesS3,
		SigningBehavior:               types.OriginAccessControlSigningBehaviorsAlways,
		SigningProtocol:               types.OriginAccessControlSigningProtocolsSigv4,
		Description:                   aws.String("create origin access control for " + name),
	}})

	if err != nil {
		return "", err
	}
	return *created.OriginAccessControl.Id, nil
}

func (c *CFClient) CreateDistributions(comment, domain string, trustedKeyGroups []string) (*types.Distribution, error) {
	callerReferenceId, _ := rand.NewUUID(rand.Reader).GetUUID()
	originId, _ := rand.NewUUID(rand.Reader).GetUUID()

	originAccessId, err := c.CreateOriginAccessControl(domain)
	if err != nil {
		return nil, err
	}

	created, err := c.Svc.CreateDistribution(context.Background(),
		&cloudfront.CreateDistributionInput{DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String(callerReferenceId),
			Comment:         aws.String(comment),
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String(originId),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyHttpsOnly,
				AllowedMethods: &types.AllowedMethods{
					Items:    []types.Method{types.MethodGet, types.MethodHead},
					Quantity: aws.Int32(2),
				},
				Compress: aws.Bool(true),
				TrustedKeyGroups: &types.TrustedKeyGroups{
					Enabled:  aws.Bool(true),
					Quantity: aws.Int32(int32(len(trustedKeyGroups))),
					Items:    trustedKeyGroups,
				},
				MinTTL: aws.Int64(0),
				ForwardedValues: &types.ForwardedValues{
					Cookies: &types.CookiePreference{
						Forward: types.ItemSelectionNone,
					},
					QueryString: aws.Bool(false),
				},
			},
			Enabled: aws.Bool(true),
			Origins: &types.Origins{
				Items: []types.Origin{
					{
						DomainName:            aws.String(domain),
						Id:                    aws.String(originId),
						OriginAccessControlId: aws.String(originAccessId),
						S3OriginConfig: &types.S3OriginConfig{
							//OriginAccessIdentity: aws.String("origin-access-identity/cloudfront/" + originAccessId),
							OriginAccessIdentity: aws.String(""),
						},
					}},
				Quantity: aws.Int32(1),
			},
			HttpVersion:   types.HttpVersionHttp2,
			IsIPV6Enabled: aws.Bool(true),
			PriceClass:    types.PriceClassPriceClassAll,
		}})
	if err != nil {
		return nil, err
	}
	return created.Distribution, nil
}

func (c *CFClient) CreatePreSignedURl(url, keyId, privateKeyPath string) (string, error) {
	expiredAt := time.Now().UTC().Add(1440 * time.Hour)

	privateKey, err := sign.LoadPEMPrivKeyFile(privateKeyPath)
	if err != nil {
		log.Println("Failed to load private key ", err)
		return "", err
	}
	urlSigner := sign.NewURLSigner(keyId, privateKey)

	generateUrl, err := urlSigner.Sign(url, expiredAt)
	if err != nil {
		return "", err
	}
	return generateUrl, nil
}
