package mappings

import (
	"fmt"
	"testing"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/jtblin/kube2iam/iam"
)

const (
	defaultBaseRole = "arn:aws:iam::123456789012:role/"
	roleKey         = "roleKey"
	namespaceKey    = "namespaceKey"
)

func TestExtractRoleARN(t *testing.T) {
	var roleExtractionTests = []struct {
		test        string
		annotations map[string]string
		defaultRole string
		expectedARN string
		expectError bool
	}{
		{
			test:        "No default, no annotation",
			annotations: map[string]string{},
			expectError: true,
		},
		{
			test:        "No default, has annotation",
			annotations: map[string]string{roleKey: "explicit-role"},
			expectedARN: "arn:aws:iam::123456789012:role/explicit-role",
		},
		{
			test:        "Default present, no annotations",
			annotations: map[string]string{},
			defaultRole: "explicit-default-role",
			expectedARN: "arn:aws:iam::123456789012:role/explicit-default-role",
		},
		{
			test:        "Default present, has annotations",
			annotations: map[string]string{roleKey: "something"},
			defaultRole: "explicit-default-role",
			expectedARN: "arn:aws:iam::123456789012:role/something",
		},
		{
			test:        "Default present, has full arn annotations",
			annotations: map[string]string{roleKey: "arn:aws:iam::999999999999:role/explicit-arn"},
			defaultRole: "explicit-default-role",
			expectedARN: "arn:aws:iam::999999999999:role/explicit-arn",
		},
		{
			test:        "Default present, has different annotations",
			annotations: map[string]string{"nonMatchingAnnotation": "something"},
			defaultRole: "explicit-default-role",
			expectedARN: "arn:aws:iam::123456789012:role/explicit-default-role",
		},
	}
	for _, tt := range roleExtractionTests {
		t.Run(tt.test, func(t *testing.T) {
			rp := RoleMapper{}
			rp.iamRoleKey = "roleKey"
			rp.defaultRoleARN = tt.defaultRole
			rp.iam = &iam.Client{BaseARN: defaultBaseRole}

			pod := &v1.Pod{}
			pod.Annotations = tt.annotations

			resp, err := rp.extractRoleARN(pod)
			if tt.expectError && err == nil {
				t.Error("Expected error however didn't recieve one")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Didn't expect error but recieved %s", err)
				return
			}
			if resp != tt.expectedARN {
				t.Errorf("Response [%s] did not equal expected [%s]", resp, tt.expectedARN)
				return
			}
		})
	}
}

func TestCheckRoleForNamespace(t *testing.T) {
	var roleCheckTests = []struct {
		test                       string
		namespaceRestriction       bool
		defaultArn                 string
		namespace                  string
		namespaceAnnotations       map[string]string
		roleARN                    string
		namespaceRestrictionFormat string
		expectedResult             bool
	}{
		{
			test:                 "No restrictions",
			namespaceRestriction: false,
			roleARN:              "arn:aws:iam::123456789012:role/explicit-role",
			namespace:            "default",
			expectedResult:       true,
		},
		// glob restrictions
		{
			test:                       "Restrictions enabled, default partial",
			namespaceRestriction:       true,
			defaultArn:                 "default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/default-role",
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, default full arn",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/default-role",
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, partial arn in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, partial glob in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/path/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"path/*\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, full arn in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, full arn with glob in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/path/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/path/*-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, full arn not in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/test-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled, no annotations",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: ""},
			namespaceRestrictionFormat: "glob",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled, multiple annotations, no match",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/test-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"explicit-role2\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled, multiple annotations, matches exact 1st",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"explicit-role2\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, multiple annotations, matches exact 2nd",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role2\", \"explicit-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, multiple annotations, matches glob 1st",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/glob-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"glob-*\", \"explicit-role\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled, multiple annotations, matches glob 2nd",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/glob-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"glob-*\"]"},
			namespaceRestrictionFormat: "glob",
			expectedResult:             true,
		},
		// regexp restrictions

		{
			test:                       "Restrictions enabled (regexp), default partial",
			namespaceRestriction:       true,
			defaultArn:                 "default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/default-role",
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), default full arn",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/default-role",
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), partial arn in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), partial regexp in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/path/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"path/.*\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), full arn in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), full arn with regexp in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/path/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/path/.*-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), full arn not in annotation",
			namespaceRestriction:       true,
			defaultArn:                 "arn:aws:iam::123456789012:role/default-role",
			roleARN:                    "arn:aws:iam::123456789012:role/test-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled (regexp), no annotations",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: ""},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled (regexp), multiple annotations, no match",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/test-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"explicit-role2\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             false,
		},
		{
			test:                       "Restrictions enabled (regexp), multiple annotations, matches exact 1st",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"explicit-role2\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), multiple annotations, matches exact 2nd",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/explicit-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role2\", \"explicit-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), multiple annotations, matches regexp 1st",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/glob-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"glob-.*\", \"explicit-role\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
		{
			test:                       "Restrictions enabled (regexp), multiple annotations, matches regexp 2nd",
			namespaceRestriction:       true,
			roleARN:                    "arn:aws:iam::123456789012:role/glob-role",
			namespace:                  "default",
			namespaceAnnotations:       map[string]string{namespaceKey: "[\"explicit-role\", \"glob-.*\"]"},
			namespaceRestrictionFormat: "regexp",
			expectedResult:             true,
		},
	}

	for _, tt := range roleCheckTests {
		t.Run(tt.test, func(t *testing.T) {
			rp := NewRoleMapper(
				roleKey,
				tt.defaultArn,
				tt.namespaceRestriction,
				namespaceKey,
				&iam.Client{BaseARN: defaultBaseRole},
				&storeMock{
					namespace:   tt.namespace,
					annotations: tt.namespaceAnnotations,
				},
				tt.namespaceRestrictionFormat,
			)

			resp := rp.checkRoleForNamespace(tt.roleARN, tt.namespace)
			if resp != tt.expectedResult {
				t.Errorf("Expected [%t] for test but recieved [%t]", tt.expectedResult, resp)
			}
		})
	}
}

type storeMock struct {
	namespace   string
	annotations map[string]string
}

func (k *storeMock) ListPodIPs() []string {
	return nil
}
func (k *storeMock) PodByIP(string) (*v1.Pod, error) {
	return nil, nil
}
func (k *storeMock) ListNamespaces() []string {
	return nil
}
func (k *storeMock) NamespaceByName(ns string) (*v1.Namespace, error) {
	if ns == k.namespace {
		nns := &v1.Namespace{}
		nns.Name = k.namespace
		nns.Annotations = k.annotations
		return nns, nil
	}
	return nil, fmt.Errorf("namespace isn't present")
}
