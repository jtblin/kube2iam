package cmd

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/karlseguin/ccache"
)

var cache = ccache.New(ccache.Configure())

const (
	ttl               = time.Minute * 15
	maxSessNameLength = 64
	fullArnPrefix     = "arn:"
)

type iam struct {
	baseARN string
}

// credentials represent the security credentials response.
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
	if strings.HasPrefix(strings.ToLower(role), fullArnPrefix) {
		return role
	}
	return fmt.Sprintf("%s%s", iam.baseARN, role)
}

func getHash(text string) string {
	h := fnv.New32a()
	_, err := h.Write([]byte(text))
	if err != nil {
		return text
	}
	return fmt.Sprintf("%x", h.Sum32())
}

// GetBaseArn get the base ARN from metadata service
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
	if len(baseArn) != 2 {
		return "", fmt.Errorf("Can't determine baseARN")
	}
	return fmt.Sprintf("%s/", baseArn[0]), nil
}

// GetInstanceIamRole get instance iam role from metadata service
func GetInstanceIamRole() (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	metadata := ec2metadata.New(sess)
	if !metadata.Available() {
		return "", fmt.Errorf("EC2 Metadata is not available, are you running on EC2?")
	}
	iamRole, err := metadata.GetMetadata("iam/security-credentials/")
	if err != nil {
		return "", err
	}
	if iamRole == "" || err != nil {
		return "", fmt.Errorf("EC2 Metadata didn't returned any IAM Role")
	}
	return iamRole, nil
}

func sessionName(roleARN, remoteIP string) string {
	idx := strings.LastIndex(roleARN, "/")
	name := fmt.Sprintf("%s-%s", getHash(remoteIP), roleARN[idx+1:])
	return fmt.Sprintf("%.[2]*[1]s", name, maxSessNameLength)
}

func (iam *iam) assumeRole(roleARN, remoteIP string) (*credentials, error) {
	item, err := cache.Fetch(roleARN, ttl, func() (interface{}, error) {
		sess, err := session.NewSession()
		if err != nil {
			return nil, err
		}
		svc := sts.New(sess, &aws.Config{LogLevel: aws.LogLevel(2)})
		resp, err := svc.AssumeRole(&sts.AssumeRoleInput{
			DurationSeconds: aws.Int64(int64(ttl.Seconds() * 2)),
			RoleArn:         aws.String(roleARN),
			RoleSessionName: aws.String(sessionName(roleARN, remoteIP)),
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
	})
	if err != nil {
		return nil, err
	}
	return item.Value().(*credentials), nil
}

func newIAM(baseARN string) *iam {
	return &iam{baseARN: baseARN}
}
