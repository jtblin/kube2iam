package kube2iam

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

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
			ns.Annotations = map[string]string{"namespaceKey": tt.annotation}
			resp := GetNamespaceRoleAnnotation(ns, "namespaceKey")

			if len(resp) != len(tt.expected) {
				t.Errorf("Expected resp length of [%d] but received [%d]", len(tt.expected), len(resp))
			}
		})
	}
}
