package iam

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
)

func stringPointer(str string) *string {
	return &str
}

var validEc2Regions = ec2.DescribeRegionsOutput{
	Regions: []types.Region{
		{
			Endpoint:   stringPointer("ec2.us-east-1.amazonaws.com"),
			RegionName: stringPointer("us-east-1"),
		},
		{
			Endpoint:   stringPointer("ec2.eu-west-1.amazonaws.com"),
			RegionName: stringPointer("eu-west-1"),
		},
	},
}

func TestIsValidRegion(t *testing.T) {
	regions := []string{
		"eu-west-1",
		"us-east-1",
	}

	for _, region := range regions {
		if !IsValidRegion(region, &validEc2Regions) {
			t.Errorf("%s is not a valid region", region)
		}
	}
}

func TestIsValidRegionWithInvalid(t *testing.T) {
	regions := []string{
		"cn-north-7",
		"",
		"xx-xxxx-x",
	}
	for _, region := range regions {
		if IsValidRegion(region, &validEc2Regions) {
			t.Errorf("%s is a valid region", region)
		}
	}
}

type MockSTSClient struct {
	AssumeRoleFunc func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

func (m *MockSTSClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return m.AssumeRoleFunc(ctx, params, optFns...)
}

func TestAssumeRole(t *testing.T) {
	// Pre-populate regions cache to avoid AWS calls
	cache.Set("awsRegions", &validEc2Regions, time.Hour)

	iamClient := &Client{
		STS: &MockSTSClient{
			AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
				return &sts.AssumeRoleOutput{
					Credentials: &ststypes.Credentials{
						AccessKeyId:     stringPointer("AKIAEXAMPLE"),
						SecretAccessKey: stringPointer("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
						SessionToken:    stringPointer("token"),
						Expiration:      aws.Time(time.Now().Add(time.Hour)),
					},
				}, nil
			},
		},
	}

	creds, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "", "1.2.3.4", time.Minute)
	if err != nil {
		t.Fatalf("AssumeRole failed: %v", err)
	}

	if creds.AccessKeyID != "AKIAEXAMPLE" {
		t.Errorf("expected AccessKeyID AKIAEXAMPLE, got %s", creds.AccessKeyID)
	}
}
