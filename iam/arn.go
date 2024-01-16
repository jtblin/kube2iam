package iam

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const fullArnPrefix = "arn:"

// ARNRegexp is the regex to check that the base ARN is valid,
// see http://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html#identifiers-arns.
var ARNRegexp = regexp.MustCompile(`^arn:(\w|-)*:iam::\d+:role\/?(\w+|-|\/|\.)*$`)

// IsValidBaseARN validates that the base ARN is valid.
func IsValidBaseARN(arn string) bool {
	return ARNRegexp.MatchString(arn)
}

// RoleARN returns the full iam role ARN.
func (iam *Client) RoleARN(role string) string {
	if strings.HasPrefix(strings.ToLower(role), fullArnPrefix) {
		return role
	}
	return fmt.Sprintf("%s%s", iam.BaseARN, role)
}

// GetBaseArn get the base ARN from metadata service.
func GetBaseArn() (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", err
	}

	client := imds.NewFromConfig(cfg)
	iamInfo, err := client.GetIAMInfo(context.TODO(), &imds.GetIAMInfoInput{})
	if err != nil {
		return "", err
	}
	arn := strings.Replace(iamInfo.IAMInfo.InstanceProfileArn, "instance-profile", "role", 1)
	baseArn := strings.Split(arn, "/")
	if len(baseArn) < 2 {
		return "", fmt.Errorf("can't determine BaseARN")
	}
	return fmt.Sprintf("%s/", baseArn[0]), nil
}
