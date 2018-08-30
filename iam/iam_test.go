package iam

import (
	"testing"
)

func TestIsValidRegion(t *testing.T) {
	regions := []string{
		"eu-central-1",
		"eu-west-1",
		"eu-west-2",
		"ap-southeast-2",
		"ap-south-1",
		"sa-east-1",
		"ca-central-1",
		"ap-northeast-1",
		"us-east-1",
		"us-west-2",
		"us-east-2",
		"ap-northeast-2",
		"ap-southeast-1",
		"us-west-1",
		"cn-north-1",
		"us-gov-west-1",
	}
	for _, region := range regions {
		if !IsValidRegion(region) {
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
		if IsValidRegion(region) {
			t.Errorf("%s is a valid region", region)
		}
	}
}
