package iam

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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

// GetInstanceIAMRole get instance IAM role from metadata service.
func GetInstanceIAMRole() (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	metadata := ec2metadata.New(sess)
	if !metadata.Available() {
		return "", errors.New("EC2 Metadata is not available, are you running on EC2?")
	}
	iamRole, err := metadata.GetMetadata("iam/security-credentials/")
	if err != nil {
		return "", err
	}
	if iamRole == "" || err != nil {
		return "", errors.New("EC2 Metadata didn't returned any IAM Role")
	}
	return iamRole, nil
}

func sessionName(roleARN, remoteIP string) string {
	idx := strings.LastIndex(roleARN, "/")
	name := fmt.Sprintf("%s-%s", getHash(remoteIP), roleARN[idx+1:])
	return fmt.Sprintf("%.[2]*[1]s", name, maxSessNameLength)
}

// Helper to format IAM return codes for metric labeling
func getIAMCode(err error) string {
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return awsErr.Code()
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
func IsValidRegion(promisedLand string) bool {
	partitions := endpoints.DefaultResolver().(endpoints.EnumPartitions).Partitions()
	for _, p := range partitions {
		for region := range p.Regions() {
			if promisedLand == region {
				return true
			}
		}
	}
	return false
}

// EndpointFor implements the endpoints.Resolver interface for use with sts
func (iam *Client) EndpointFor(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	// only for sts service
	if service == "sts" {
		// only if a valid region is explicitly set
		if IsValidRegion(region) {
			iam.Endpoint = GetEndpointFromRegion(region)
			return endpoints.ResolvedEndpoint{
				URL:           iam.Endpoint,
				SigningRegion: region,
			}, nil
		}
	}
	return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
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

		sess, err := session.NewSession()
		if err != nil {
			return nil, err
		}
		config := aws.NewConfig().WithLogLevel(2)
		if iam.UseRegionalEndpoint {
			config = config.WithEndpointResolver(iam)
		}
		svc := sts.New(sess, config)
		assumeRoleInput := sts.AssumeRoleInput{
			DurationSeconds: aws.Int64(int64(sessionTTL.Seconds() * 2)),
			RoleArn:         aws.String(roleARN),
			RoleSessionName: aws.String(sessionName(roleARN, remoteIP)),
		}
		// Only inject the externalID if one was provided with the request
		if externalID != "" {
			assumeRoleInput.SetExternalId(externalID)
		}
		resp, err := svc.AssumeRole(&assumeRoleInput)
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
