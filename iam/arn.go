package iam

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
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
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	metadata := ec2metadata.New(sess)
	if !metadata.Available() {
		return "", fmt.Errorf("EC2 Metadata is not available, are you running on EC2?")
	}
	iamInfo, err := metadata.IAMInfo()
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
