package iam

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	smithy "github.com/aws/smithy-go"
	"github.com/karlseguin/ccache"
)

// ---- helpers ----------------------------------------------------------------

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

// ---- mock clients -----------------------------------------------------------

type MockSTSClient struct {
	AssumeRoleFunc func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

func (m *MockSTSClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return m.AssumeRoleFunc(ctx, params, optFns...)
}

type MockRegionClient struct {
	DescribeRegionsFunc func(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

func (m *MockRegionClient) DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return m.DescribeRegionsFunc(ctx, params, optFns...)
}

// MockIMDSClient implements IMDSClient for testing.
type MockIMDSClient struct {
	GetMetadataFunc func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
	GetIAMInfoFunc  func(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error)
}

func (m *MockIMDSClient) GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	return m.GetMetadataFunc(ctx, params, optFns...)
}

func (m *MockIMDSClient) GetIAMInfo(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
	return m.GetIAMInfoFunc(ctx, params, optFns...)
}

// mockMetadataOutput builds an imds.GetMetadataOutput with the given string body.
func mockMetadataOutput(body string) *imds.GetMetadataOutput {
	return &imds.GetMetadataOutput{Content: io.NopCloser(bytes.NewBufferString(body))}
}

// ---- Region tests -----------------------------------------------------------

func TestIsValidRegion(t *testing.T) {
	regions := []string{"eu-west-1", "us-east-1"}
	for _, region := range regions {
		if !IsValidRegion(region, &validEc2Regions) {
			t.Errorf("%s is not a valid region", region)
		}
	}
}

func TestIsValidRegionWithInvalid(t *testing.T) {
	regions := []string{"cn-north-7", "", "xx-xxxx-x"}
	for _, region := range regions {
		if IsValidRegion(region, &validEc2Regions) {
			t.Errorf("%s should not be a valid region", region)
		}
	}
}

// ---- Utility function tests -------------------------------------------------

func TestGetHash(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"1.2.3.4"},
		{"some-long-string-with-special-chars-!@#"},
	}
	for _, tt := range tests {
		h := getHash(tt.input)
		if h == "" {
			t.Errorf("getHash(%q) returned empty string", tt.input)
		}
		// deterministic
		if getHash(tt.input) != h {
			t.Errorf("getHash(%q) is not deterministic", tt.input)
		}
	}
}

func TestGetHashDifferentInputsDifferentOutputs(t *testing.T) {
	if getHash("1.2.3.4") == getHash("5.6.7.8") {
		t.Error("expected different hashes for different inputs")
	}
}

func TestSessionName(t *testing.T) {
	tests := []struct {
		name        string
		roleARN     string
		remoteIP    string
		wantMaxLen  int
		wantContain string
	}{
		{
			name:        "short role",
			roleARN:     "arn:aws:iam::123456789012:role/my-role",
			remoteIP:    "1.2.3.4",
			wantMaxLen:  maxSessNameLength,
			wantContain: "my-role",
		},
		{
			name:       "very long role truncated to 64",
			roleARN:    "arn:aws:iam::123456789012:role/" + strings.Repeat("x", 100),
			remoteIP:   "1.2.3.4",
			wantMaxLen: maxSessNameLength,
		},
		{
			name:        "role with path",
			roleARN:     "arn:aws:iam::123456789012:role/path/to/role-name",
			remoteIP:    "10.0.0.1",
			wantMaxLen:  maxSessNameLength,
			wantContain: "role-name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sessionName(tt.roleARN, tt.remoteIP)
			if len(result) > tt.wantMaxLen {
				t.Errorf("sessionName too long: got %d, want <= %d", len(result), tt.wantMaxLen)
			}
			if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
				t.Errorf("sessionName %q does not contain %q", result, tt.wantContain)
			}
		})
	}
}

func TestGetEndpointFromRegion(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-east-1", "https://sts.us-east-1.amazonaws.com"},
		{"eu-west-1", "https://sts.eu-west-1.amazonaws.com"},
		{"cn-north-1", "https://sts.cn-north-1.amazonaws.com.cn"},
		{"cn-northwest-1", "https://sts.cn-northwest-1.amazonaws.com.cn"},
	}
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := GetEndpointFromRegion(tt.region)
			if got != tt.expected {
				t.Errorf("GetEndpointFromRegion(%q) = %q, want %q", tt.region, got, tt.expected)
			}
		})
	}
}

// mockAPIError implements smithy.APIError for testing getIAMCode.
type mockAPIError struct {
	code string
}

func (e *mockAPIError) Error() string        { return e.code }
func (e *mockAPIError) ErrorCode() string    { return e.code }
func (e *mockAPIError) ErrorMessage() string { return e.code }
func (e *mockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultUnknown
}

func TestGetIAMCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error returns success", nil, "Success"},
		{"smithy API error returns code", &mockAPIError{code: "AccessDenied"}, "AccessDenied"},
		{"generic error returns unknown", errors.New("some error"), "UnknownError"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIAMCode(tt.err)
			if got != tt.expected {
				t.Errorf("getIAMCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---- AssumeRole tests -------------------------------------------------------

func newTestIAMClient() *Client {
	return &Client{
		Cache: ccache.New(ccache.Configure()),
		Region: &MockRegionClient{
			DescribeRegionsFunc: func(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
				return &validEc2Regions, nil
			},
		},
	}
}

func TestAssumeRole(t *testing.T) {
	iamClient := newTestIAMClient()
	iamClient.STS = &MockSTSClient{
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
	}

	creds, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "", "1.2.3.4", time.Minute)
	if err != nil {
		t.Fatalf("AssumeRole failed: %v", err)
	}
	if creds.AccessKeyID != "AKIAEXAMPLE" {
		t.Errorf("expected AccessKeyID AKIAEXAMPLE, got %s", creds.AccessKeyID)
	}
	if creds.Code != "Success" {
		t.Errorf("expected Code Success, got %s", creds.Code)
	}
}

func TestAssumeRoleWithExternalID(t *testing.T) {
	var capturedInput *sts.AssumeRoleInput
	iamClient := newTestIAMClient()
	iamClient.STS = &MockSTSClient{
		AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			capturedInput = params
			return &sts.AssumeRoleOutput{
				Credentials: &ststypes.Credentials{
					AccessKeyId:     stringPointer("AKIAEXAMPLE"),
					SecretAccessKey: stringPointer("secret"),
					SessionToken:    stringPointer("token"),
					Expiration:      aws.Time(time.Now().Add(time.Hour)),
				},
			}, nil
		},
	}

	_, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "my-external-id", "1.2.3.4", time.Minute)
	if err != nil {
		t.Fatalf("AssumeRole failed: %v", err)
	}
	if capturedInput.ExternalId == nil || *capturedInput.ExternalId != "my-external-id" {
		t.Errorf("expected ExternalId to be set to 'my-external-id', got %v", capturedInput.ExternalId)
	}
}

func TestAssumeRoleWithoutExternalIDNotSet(t *testing.T) {
	var capturedInput *sts.AssumeRoleInput
	iamClient := newTestIAMClient()
	iamClient.STS = &MockSTSClient{
		AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			capturedInput = params
			return &sts.AssumeRoleOutput{
				Credentials: &ststypes.Credentials{
					AccessKeyId:     stringPointer("AKIAEXAMPLE"),
					SecretAccessKey: stringPointer("secret"),
					SessionToken:    stringPointer("token"),
					Expiration:      aws.Time(time.Now().Add(time.Hour)),
				},
			}, nil
		},
	}

	_, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "", "1.2.3.4", time.Minute)
	if err != nil {
		t.Fatalf("AssumeRole failed: %v", err)
	}
	if capturedInput.ExternalId != nil {
		t.Errorf("expected ExternalId to be nil when not provided, got %v", *capturedInput.ExternalId)
	}
}

func TestAssumeRoleError(t *testing.T) {
	iamClient := newTestIAMClient()
	iamClient.STS = &MockSTSClient{
		AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			return nil, errors.New("STS unavailable")
		},
	}

	_, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "", "1.2.3.4", time.Minute)
	if err == nil {
		t.Fatal("expected error from AssumeRole but got nil")
	}
}

func TestAssumeRoleCache(t *testing.T) {
	callCount := 0
	iamClient := newTestIAMClient()
	iamClient.STS = &MockSTSClient{
		AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
			callCount++
			return &sts.AssumeRoleOutput{
				Credentials: &ststypes.Credentials{
					AccessKeyId:     stringPointer("AKIAEXAMPLE"),
					SecretAccessKey: stringPointer("secret"),
					SessionToken:    stringPointer("token"),
					Expiration:      aws.Time(time.Now().Add(time.Hour)),
				},
			}, nil
		},
	}

	roleARN := "arn:aws:iam::123456789012:role/cached-role"
	for i := 0; i < 3; i++ {
		_, err := iamClient.AssumeRole(roleARN, "", "1.2.3.4", time.Hour)
		if err != nil {
			t.Fatalf("AssumeRole call %d failed: %v", i, err)
		}
	}

	if callCount != 1 {
		t.Errorf("expected STS to be called once for cached role, got %d calls", callCount)
	}
}

func TestAssumeRoleRegionError(t *testing.T) {
	iamClient := &Client{
		Cache: ccache.New(ccache.Configure()),
		Region: &MockRegionClient{
			DescribeRegionsFunc: func(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
				return nil, errors.New("describe regions failed")
			},
		},
		STS: &MockSTSClient{
			AssumeRoleFunc: func(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
				return nil, nil // should not be reached
			},
		},
	}

	_, err := iamClient.AssumeRole("arn:aws:iam::123456789012:role/role", "", "1.2.3.4", time.Minute)
	if err == nil {
		t.Fatal("expected error when DescribeRegions fails")
	}
}

// ---- IMDS / GetInstanceId tests ---------------------------------------------

func TestGetInstanceId(t *testing.T) {
	iamClient := &Client{
		IMDS: &MockIMDSClient{
			GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
				if params.Path != "instance-id" {
					return nil, errors.New("unexpected path: " + params.Path)
				}
				return mockMetadataOutput("i-0abcdef1234567890"), nil
			},
		},
	}

	id, err := iamClient.GetInstanceId()
	if err != nil {
		t.Fatalf("GetInstanceId failed: %v", err)
	}
	if id != "i-0abcdef1234567890" {
		t.Errorf("expected instance ID i-0abcdef1234567890, got %s", id)
	}
}

func TestGetInstanceIdError(t *testing.T) {
	iamClient := &Client{
		IMDS: &MockIMDSClient{
			GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
				return nil, errors.New("IMDS unavailable")
			},
		},
	}

	_, err := iamClient.GetInstanceId()
	if err == nil {
		t.Fatal("expected error from GetInstanceId but got nil")
	}
}

func TestGetMetadataPathEmptyBody(t *testing.T) {
	mockClient := &MockIMDSClient{
		GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return mockMetadataOutput(""), nil
		},
	}

	_, err := getMetadataPath(mockClient, "instance-id")
	if err == nil {
		t.Fatal("expected error for empty metadata body")
	}
}
