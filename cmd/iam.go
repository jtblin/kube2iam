package cmd

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

type iam struct {
	baseARN string
}

// Credentials represent the security credentials response
type credentials struct {
	Code            string
	LastUpdated     string
	Type            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      string
}

func (iam *iam) roleARN(role string) string {
	return fmt.Sprintf("%s%s", iam.baseARN, role)
}

func (iam *iam) assumeRole(roleARN string) (*credentials, error) {
	svc := sts.New(session.New(), &aws.Config{LogLevel: aws.LogLevel(2)})
	resp, err := svc.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String("kube2iam"), // TODO: generate random session name
	})
	if err != nil {
		return nil, err
	}
	return &credentials{
		AccessKeyID:     *resp.Credentials.AccessKeyId,
		Code:            "Success",
		Expiration:      resp.Credentials.Expiration.Format("2006-01-02T15:04:05Z"),
		LastUpdated:     time.Now().Format("2006-01-02T15:04:05Z"),
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		Token:           *resp.Credentials.SessionToken,
		Type:            "AWS-HMAC",
	}, nil
}

func newIAM(baseARN string) *iam {
	return &iam{baseARN: baseARN}
}
