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

type iam struct {
	baseARN    string
	cache      *ccache.Cache
	ttl        time.Duration
	awsSession *session.Session
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

func (iam *iam) cacheCredentials(roleARN, remoteIP string, credentials *credentials) {
	itemKey := fmt.Sprintf("%s-%s", roleARN, getHash(remoteIP))
	// Cache for the desired time - 5 minutes.
	// The refresher will attempt to refresh creds 10 minutes before the creds expire.
	iam.cache.Set(itemKey, credentials, iam.ttl-(time.Duration(5)*time.Minute))
}

func (iam *iam) assumeRoleNoCache(roleARN, remoteIP string) (*credentials, error) {
	idx := strings.LastIndex(roleARN, "/")
	svc := sts.New(iam.awsSession, &aws.Config{LogLevel: aws.LogLevel(2)})
	resp, err := svc.AssumeRole(&sts.AssumeRoleInput{
		DurationSeconds: aws.Int64(int64(iam.ttl.Seconds())),
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
}

func (iam *iam) assumeRole(roleARN, remoteIP string) (*credentials, error) {
	itemKey := fmt.Sprintf("%s-%s", roleARN, getHash(remoteIP))
	item, err := iam.cache.Fetch(itemKey, iam.ttl, func() (interface{}, error) {
		return iam.assumeRoleNoCache(roleARN, remoteIP)
	})
	if err != nil {
		return nil, err
	}
	return item.Value().(*credentials), nil
}

func newIAM(baseARN string, ttl int) *iam {
	return &iam{
		baseARN:    baseARN,
		ttl:        time.Second * time.Duration(ttl),
		cache:      ccache.New(ccache.Configure()),
		awsSession: session.New(),
	}
}
