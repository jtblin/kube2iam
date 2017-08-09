package processor

import (
	"fmt"
	"testing"

	"github.com/jtblin/kube2iam/iam"
	"k8s.io/client-go/pkg/api/v1"
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
			rp := RoleProcessor{}
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
		test                 string
		namespaceRestriction bool
		defaultArn           string
		namespace            string
		namespaceAnnotations map[string]string
		roleARN              string
		expectedResult       bool
	}{
		{
			test:                 "No restrictions",
			namespaceRestriction: false,
			roleARN:              "arn:aws:iam::123456789012:role/explicit-role",
			namespace:            "default",
			expectedResult:       true,
		},
		{
			test:                 "Restrictions enabled, default partial",
			namespaceRestriction: true,
			defaultArn:           "default-role",
			roleARN:              "arn:aws:iam::123456789012:role/default-role",
			expectedResult:       true,
		},
		{
			test:                 "Restrictions enabled, default full arn",
			namespaceRestriction: true,
			defaultArn:           "arn:aws:iam::123456789012:role/default-role",
			roleARN:              "arn:aws:iam::123456789012:role/default-role",
			expectedResult:       true,
		},
		{
			test:                 "Restrictions enabled, partial arn in annotation",
			namespaceRestriction: true,
			defaultArn:           "arn:aws:iam::123456789012:role/default-role",
			roleARN:              "arn:aws:iam::123456789012:role/explicit-role",
			namespace:            "default",
			namespaceAnnotations: map[string]string{namespaceKey: "[\"explicit-role\"]"},
			expectedResult:       true,
		},
		{
			test:                 "Restrictions enabled, full arn in annotation",
			namespaceRestriction: true,
			defaultArn:           "arn:aws:iam::123456789012:role/default-role",
			roleARN:              "arn:aws:iam::123456789012:role/explicit-role",
			namespace:            "default",
			namespaceAnnotations: map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			expectedResult:       true,
		},
		{
			test:                 "Restrictions enabled, full arn not in annotation",
			namespaceRestriction: true,
			defaultArn:           "arn:aws:iam::123456789012:role/default-role",
			roleARN:              "arn:aws:iam::123456789012:role/test-role",
			namespace:            "default",
			namespaceAnnotations: map[string]string{namespaceKey: "[\"arn:aws:iam::123456789012:role/explicit-role\"]"},
			expectedResult:       false,
		},
		{
			test:                 "Restrictions enabled, no annotations",
			namespaceRestriction: true,
			roleARN:              "arn:aws:iam::123456789012:role/explicit-role",
			namespace:            "default",
			namespaceAnnotations: map[string]string{namespaceKey: ""},
			expectedResult:       false,
		},
	}

	for _, tt := range roleCheckTests {
		t.Run(tt.test, func(t *testing.T) {
			rp := NewRoleProcessor(
				roleKey,
				tt.defaultArn,
				tt.namespaceRestriction,
				namespaceKey,
				&iam.Client{BaseARN: defaultBaseRole},
				&kubeStoreMock{
					namespace:   tt.namespace,
					annotations: tt.namespaceAnnotations,
				},
			)

			resp := rp.checkRoleForNamespace(tt.roleARN, tt.namespace)
			if resp != tt.expectedResult {
				t.Errorf("Expected [%t] for test but recieved [%t]", tt.expectedResult, resp)
			}
		})
	}
}

func TestGetNamespaceRoleAnnotation(t *testing.T) {
	var parseTests = []struct {
		test       string
		annotation string
		expected   []string
	}{
		{
			test:       "Empty string",
			annotation: "",
			expected:   []string{},
		},
		{
			test:       "Malformed string",
			annotation: "something maleformed here",
			expected:   []string{},
		},
		{
			test:       "Single entity array",
			annotation: `["test-something"]`,
			expected:   []string{"test-something"},
		},
		{
			test:       "Multi-element array",
			annotation: `["test-something","test-another"]`,
			expected:   []string{"test-something", "test-another"},
		},
	}

	for _, tt := range parseTests {
		t.Run(tt.test, func(t *testing.T) {
			ns := &v1.Namespace{}
			ns.Annotations = map[string]string{namespaceKey: tt.annotation}
			resp := GetNamespaceRoleAnnotation(ns, namespaceKey)

			if len(resp) != len(tt.expected) {
				t.Errorf("Expected resp length of [%d] but received [%d]", len(tt.expected), len(resp))
			}
		})
	}
}

type kubeStoreMock struct {
	namespace   string
	annotations map[string]string
}

func (k *kubeStoreMock) ListPodIPs() []string {
	return nil
}
func (k *kubeStoreMock) PodByIP(string) (*v1.Pod, error) {
	return nil, nil
}
func (k *kubeStoreMock) ListNamespaces() []string {
	return nil
}
func (k *kubeStoreMock) NamespaceByName(ns string) (*v1.Namespace, error) {
	if ns == k.namespace {
		nns := &v1.Namespace{}
		nns.Name = k.namespace
		nns.Annotations = k.annotations
		return nns, nil
	}
	return nil, fmt.Errorf("Namepsace isn't present")
}
