package iam

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
