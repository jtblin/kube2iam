package cmd

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/karlseguin/ccache"
)

var cache = ccache.New(ccache.Configure())

const (
	ttl = time.Minute * 15
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
	return fmt.Sprintf("%s%s", iam.baseARN, role)
}

func getHash(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum32())
}

func (iam *iam) assumeRole(roleARN, remoteIP string) (*credentials, error) {
	item, err := cache.Fetch(roleARN, ttl, func() (interface{}, error) {
		idx := strings.LastIndex(roleARN, "/")
		svc := sts.New(session.New(), &aws.Config{LogLevel: aws.LogLevel(2)})
		resp, err := svc.AssumeRole(&sts.AssumeRoleInput{
			DurationSeconds: aws.Int64(int64(ttl.Seconds() * 2)),
			RoleArn:         aws.String(roleARN),
			RoleSessionName: aws.String(fmt.Sprintf("%s-%s", roleARN[idx+1:], getHash(remoteIP))),
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
