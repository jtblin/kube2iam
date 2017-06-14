package iam

import (
	"testing"
)

func TestIsValidBaseARN(t *testing.T) {
	arns := []string{
		"arn:aws:iam::123456789012:role/path",
		"arn:aws:iam::123456789012:role/path/",
		"arn:aws:iam::123456789012:role/path/sub-path",
		"arn:aws:iam::123456789012:role/path/sub_path",
		"arn:aws:iam::123456789012:role",
		"arn:aws:iam::123456789012:role/",
		"arn:aws:iam::123456789012:role-part",
		"arn:aws:iam::123456789012:role_part",
		"arn:aws:iam::123456789012:role_123",
		"arn:aws-us-gov:iam::123456789012:role",
	}
	for _, arn := range arns {
		if !IsValidBaseARN(arn) {
			t.Errorf("%s is a valid base arn", arn)
		}
	}
}

func TestIsValidBaseARNWithInvalid(t *testing.T) {
	arns := []string{
		"arn:aws:iam::123456789012::role/path",
		"arn:aws:iam:us-east-1:123456789012:role/path",
		"arn:aws:s3::123456789012:role/path",
		"arn:aws:iam::123456789012:role/$",
		"arn:aws:iam::12345-6789012:role/",
		"arn:aws:iam::abcdef:role/",
		"arn:aws:iam:::role",
	}
	for _, arn := range arns {
		if IsValidBaseARN(arn) {
			t.Errorf("%s is not a valid base arn", arn)
		}
	}
}
