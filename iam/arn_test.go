package iam

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

func TestIsValidBaseARN(t *testing.T) {
	valid := []string{
		"arn:aws:iam::123456789012:role/my-role",
		"arn:aws:iam::123456789012:role/path/to/role",
		"arn:aws-cn:iam::123456789012:role/my-role",
		"arn:aws:iam::123456789012:role/my.role",
		"arn:aws:iam::123456789012:role/",
	}
	for _, arn := range valid {
		t.Run(arn, func(t *testing.T) {
			if !IsValidBaseARN(arn) {
				t.Errorf("expected %q to be a valid base ARN", arn)
			}
		})
	}
}

func TestIsValidBaseARNInvalid(t *testing.T) {
	invalid := []string{
		"",
		"not-an-arn",
		"arn:aws:s3:::my-bucket",
		"arn:aws:iam::123456789012:user/my-user",
		"arn:aws:iam:us-east-1:123456789012:role/my-role",
	}
	for _, arn := range invalid {
		t.Run(arn, func(t *testing.T) {
			if IsValidBaseARN(arn) {
				t.Errorf("expected %q to be an invalid base ARN", arn)
			}
		})
	}
}

func TestRoleARN(t *testing.T) {
	client := &Client{BaseARN: "arn:aws:iam::123456789012:role/"}

	tests := []struct {
		name     string
		role     string
		expected string
	}{
		{
			name:     "short role name gets base ARN prepended",
			role:     "my-role",
			expected: "arn:aws:iam::123456789012:role/my-role",
		},
		{
			name:     "full ARN is returned unchanged",
			role:     "arn:aws:iam::999999999999:role/cross-account",
			expected: "arn:aws:iam::999999999999:role/cross-account",
		},
		{
			name:     "ARN prefix check is case-insensitive",
			role:     "ARN:aws:iam::999999999999:role/upper-case",
			expected: "ARN:aws:iam::999999999999:role/upper-case",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.RoleARN(tt.role)
			if got != tt.expected {
				t.Errorf("RoleARN(%q) = %q, want %q", tt.role, got, tt.expected)
			}
		})
	}
}

func TestRoleARNNoBaseARN(t *testing.T) {
	client := &Client{BaseARN: ""}
	got := client.RoleARN("my-role")
	if got != "my-role" {
		t.Errorf("expected bare role name when no BaseARN, got %q", got)
	}
}

func TestGetBaseArnWithClient(t *testing.T) {
	mockIMDS := &MockIMDSClient{
		GetIAMInfoFunc: func(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
			return &imds.GetIAMInfoOutput{
				IAMInfo: imds.IAMInfo{
					InstanceProfileArn: "arn:aws:iam::123456789012:instance-profile/my-role",
				},
			}, nil
		},
		// GetMetadata is not used by GetBaseArnWithClient
		GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return &imds.GetMetadataOutput{Content: io.NopCloser(bytes.NewBufferString(""))}, nil
		},
	}

	arn, err := GetBaseArnWithClient(mockIMDS)
	if err != nil {
		t.Fatalf("GetBaseArnWithClient failed: %v", err)
	}
	expected := "arn:aws:iam::123456789012:role/"
	if arn != expected {
		t.Errorf("expected %q, got %q", expected, arn)
	}
}

func TestGetBaseArnWithClientIMDSError(t *testing.T) {
	mockIMDS := &MockIMDSClient{
		GetIAMInfoFunc: func(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
			return nil, errors.New("IMDS unavailable")
		},
		GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return &imds.GetMetadataOutput{Content: io.NopCloser(bytes.NewBufferString(""))}, nil
		},
	}

	_, err := GetBaseArnWithClient(mockIMDS)
	if err == nil {
		t.Fatal("expected error from GetBaseArnWithClient when IMDS fails")
	}
}

func TestGetBaseArnWithClientMalformedARN(t *testing.T) {
	mockIMDS := &MockIMDSClient{
		GetIAMInfoFunc: func(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
			return &imds.GetIAMInfoOutput{
				IAMInfo: imds.IAMInfo{
					// No "/" separator — malformed ARN
					InstanceProfileArn: "arn:aws:iam::123456789012:instance-profile",
				},
			}, nil
		},
		GetMetadataFunc: func(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
			return &imds.GetMetadataOutput{Content: io.NopCloser(bytes.NewBufferString(""))}, nil
		},
	}

	_, err := GetBaseArnWithClient(mockIMDS)
	if err == nil {
		t.Fatal("expected error for malformed ARN with no path segment")
	}
	if !strings.Contains(err.Error(), "can't determine BaseARN") {
		t.Errorf("unexpected error message: %v", err)
	}
}
