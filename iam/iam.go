package iam

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithy "github.com/aws/smithy-go"
	"github.com/jtblin/kube2iam/metrics"
	"github.com/karlseguin/ccache"
)

var cache = ccache.New(ccache.Configure())

const (
	maxSessNameLength = 64
)

// Client represents an IAM client.
type Client struct {
	BaseARN             string
	Endpoint            string
	UseRegionalEndpoint bool
}

// Credentials represent the security Credentials response.
type Credentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	Code            string
	Expiration      string
	LastUpdated     string
	SecretAccessKey string
	Token           string
	Type            string
}

func getHash(text string) string {
	h := fnv.New32a()
	_, err := h.Write([]byte(text))
	if err != nil {
		return text
	}
	return fmt.Sprintf("%x", h.Sum32())
}

func getInstanceMetadata(path string) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", err
	}

	client := imds.NewFromConfig(cfg)
	metadataResult, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: path,
	})
	if err != nil {
		return "", errors.New(fmt.Sprintf("EC2 Metadata [%s] response error, got %v", err, path))
	}
	// https://aws.github.io/aws-sdk-go-v2/docs/making-requests/#responses-with-ioreadcloser
	defer metadataResult.Content.Close()
	instanceId, err := ioutil.ReadAll(metadataResult.Content)

	if err != nil {
		return "", errors.New(fmt.Sprintf("Expect to read content [%s] from bytes, got %v", err, path))
	}

	if string(instanceId) == "" {
		return "", errors.New(fmt.Sprintf("EC2 Metadata didn't returned [%s], got empty string", path))
	}
	return string(instanceId), nil
}

// GetInstanceIAMRole get instance IAM role from metadata service.
func GetInstanceIAMRole() (string, error) {
	iamRole, err := getInstanceMetadata("iam/security-credentials/")

	if err == nil {
		return "", err
	}
	return string(iamRole), nil
}

// Get InstanceId for healthcheck
func (iam *Client) GetInstanceId() (string, error) {
	instanceId, err := getInstanceMetadata("instance-id")

	if err == nil {
		return "", err
	}
	return string(instanceId), nil
}

func sessionName(roleARN, remoteIP string) string {
	idx := strings.LastIndex(roleARN, "/")
	name := fmt.Sprintf("%s-%s", getHash(remoteIP), roleARN[idx+1:])
	return fmt.Sprintf("%.[2]*[1]s", name, maxSessNameLength)
}

// Helper to format IAM return codes for metric labeling
//
// https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/#api-error-responses
// All service API response errors implement the smithy.APIError interface type.
// This interface can be used to handle both modeled or un-modeled service error responses
func getIAMCode(err error) string {
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			return apiErr.ErrorCode()
		}
		return metrics.IamUnknownFailCode
	}
	return metrics.IamSuccessCode
}

// GetEndpointFromRegion formas a standard sts endpoint url given a region
func GetEndpointFromRegion(region string) string {
	endpoint := fmt.Sprintf("https://sts.%s.amazonaws.com", region)
	if strings.HasPrefix(region, "cn-") {
		endpoint = fmt.Sprintf("https://sts.%s.amazonaws.com.cn", region)
	}
	return endpoint
}

// IsValidRegion tests for a vaild region name
func IsValidRegion(promisedLand string, regions *ec2.DescribeRegionsOutput) bool {
	for _, region := range regions.Regions {
		if promisedLand == *region.RegionName {
			return true
		}
	}
	return false
}

// Regions list to validate input region name
//
// https://stackoverflow.com/a/69935735/3945261
func loadRegions() (*ec2.DescribeRegionsOutput, error) {
	regionsCache, err := cache.Fetch("awsRegions", time.Hour*24*30, func() (interface{}, error) {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return nil, err
		}
		ec2Client := ec2.NewFromConfig(cfg)
		r, err := ec2Client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})
		if err != nil {
			return nil, err
		}

		return r, nil
	})

	if err != nil {
		return nil, err
	}

	return regionsCache.Value().(*ec2.DescribeRegionsOutput), nil
}

// AssumeRole returns an IAM role Credentials using AWS STS.
func (iam *Client) AssumeRole(roleARN, externalID string, remoteIP string, sessionTTL time.Duration) (*Credentials, error) {
	hitCache := true
	item, err := cache.Fetch(roleARN, sessionTTL, func() (interface{}, error) {
		hitCache = false

		// Set up a prometheus timer to track the AWS request duration. It stores the timer value when
		// observed. A function gets err at observation time to report the status of the request after the function returns.
		var err error
		lvsProducer := func() []string {
			return []string{getIAMCode(err), roleARN}
		}
		timer := metrics.NewFunctionTimer(metrics.IamRequestSec, lvsProducer, nil)
		defer timer.ObserveDuration()

		regions, err := loadRegions()
		if err != nil {
			return nil, err
		}

		var customSTSResolver = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == sts.ServiceID && IsValidRegion(region, regions) {
				return aws.Endpoint{
					URL:           GetEndpointFromRegion(region),
					SigningRegion: region,
				}, nil
			}

			// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})

		cfg, err := config.LoadDefaultConfig(
			context.TODO(),
			config.WithEndpointResolverWithOptions(customSTSResolver),
		)
		if err != nil {
			return nil, err
		}
		svc := sts.NewFromConfig(cfg)
		assumeRoleInput := sts.AssumeRoleInput{
			DurationSeconds: aws.Int32(int32(sessionTTL.Seconds() * 2)),
			RoleArn:         aws.String(roleARN),
			RoleSessionName: aws.String(sessionName(roleARN, remoteIP)),
		}
		// Only inject the externalID if one was provided with the request
		if externalID != "" {
			assumeRoleInput.ExternalId = aws.String(externalID)
		}

		// Maybe use NewAssumeRoleProvider - https://github.com/aws/aws-sdk-go-v2/blob/credentials/v1.12.10/credentials/stscreds/assume_role_provider.go#L254
		// That's wrapper for AssumeRole with some default values for options
		// https://github.com/aws/aws-sdk-go-v2/blob/credentials/v1.12.10/credentials/stscreds/assume_role_provider.go#L270
		resp, err := svc.AssumeRole(context.TODO(), &assumeRoleInput)
		if err != nil {
			return nil, err
		}

		return &Credentials{
			AccessKeyID:     *resp.Credentials.AccessKeyId,
			Code:            "Success",
			Expiration:      resp.Credentials.Expiration.Format("2006-01-02T15:04:05Z"),
			LastUpdated:     time.Now().Format("2006-01-02T15:04:05Z"),
			SecretAccessKey: *resp.Credentials.SecretAccessKey,
			Token:           *resp.Credentials.SessionToken,
			Type:            "AWS-HMAC",
		}, nil
	})
	if hitCache {
		metrics.IamCacheHitCount.WithLabelValues(roleARN).Inc()
	}
	if err != nil {
		return nil, err
	}
	return item.Value().(*Credentials), nil
}

// NewClient returns a new IAM client.
func NewClient(baseARN string, regional bool) *Client {
	return &Client{
		BaseARN:             baseARN,
		Endpoint:            "sts.amazonaws.com",
		UseRegionalEndpoint: regional,
	}
}
