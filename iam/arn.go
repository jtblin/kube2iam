package iam

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

// GetBaseArn gets the base ARN from the metadata service.
// If client is nil, a default IMDS client is created.
func GetBaseArn() (string, error) {
	return GetBaseArnWithClient(nil)
}

// GetBaseArnWithClient gets the base ARN using the provided IMDSClient.
// This allows injection of a mock for testing.
func GetBaseArnWithClient(client IMDSClient) (string, error) {
	if client == nil {
		var err error
		client, err = newIMDSClient()
		if err != nil {
			return "", err
		}
	}

	iamInfo, err := client.GetIAMInfo(context.TODO(), &imds.GetIAMInfoInput{})
	if err != nil {
		return "", err
	}
	arn := strings.Replace(iamInfo.InstanceProfileArn, "instance-profile", "role", 1)
	baseArn := strings.Split(arn, "/")
	if len(baseArn) < 2 {
		return "", fmt.Errorf("can't determine BaseARN")
	}
	return fmt.Sprintf("%s/", baseArn[0]), nil
}
