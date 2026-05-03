package mappings

import (
	"fmt"
	"testing"

	"github.com/jtblin/kube2iam/iam"
	v1 "k8s.io/api/core/v1"
)

const (
	defaultBaseRole = "arn:aws:iam::123456789012:role/"
	roleKey         = "roleKey"
	externalIDKey   = "externalIDKey"
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
		{
			test:        "Default present, has annotations, has externalID",
			annotations: map[string]string{roleKey: "something", externalIDKey: "externalID"},
			defaultRole: "explicit-default-role",
			expectedARN: "arn:aws:iam::123456789012:role/something",
		},
	}
	for _, tt := range roleExtractionTests {
		t.Run(tt.test, func(t *testing.T) {
			rp := RoleMapper{}
			rp.iamRoleKey = "roleKey"
			rp.iamExternalIDKey = "externalIDKey"
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
		externalID                 string
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
				externalIDKey,
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

	// Extended fields for GetRoleMapping / GetExternalIDMapping tests.
	pods   map[string]*v1.Pod
	podErr error
	nsList []string
	nsMap  map[string]*v1.Namespace
}

func (k *storeMock) ListPodIPs() []string {
	if k.pods != nil {
		ips := make([]string, 0, len(k.pods))
		for ip := range k.pods {
			ips = append(ips, ip)
		}
		return ips
	}
	return nil
}

func (k *storeMock) PodByIP(ip string) (*v1.Pod, error) {
	if k.podErr != nil {
		return nil, k.podErr
	}
	if k.pods != nil {
		if pod, ok := k.pods[ip]; ok {
			return pod, nil
		}
		return nil, fmt.Errorf("pod with specified IP not found")
	}
	return nil, nil
}

func (k *storeMock) ListNamespaces() []string {
	if k.nsList != nil {
		return k.nsList
	}
	return nil
}

func (k *storeMock) NamespaceByName(ns string) (*v1.Namespace, error) {
	if k.nsMap != nil {
		if n, ok := k.nsMap[ns]; ok {
			return n, nil
		}
		return nil, fmt.Errorf("namespace isn't present")
	}
	if ns == k.namespace {
		nns := &v1.Namespace{}
		nns.Name = k.namespace
		nns.Annotations = k.annotations
		return nns, nil
	}
	return nil, fmt.Errorf("namespace isn't present")
}

// ---- GetRoleMapping tests ---------------------------------------------------

func TestGetRoleMappingNoAnnotationNoDefault(t *testing.T) {
	pod := &v1.Pod{}
	pod.Status.PodIP = "10.0.0.1"
	store := &storeMock{pods: map[string]*v1.Pod{"10.0.0.1": pod}}

	// No defaultRole, and BaseARN is also empty — so RoleARN("") == "" and extractRoleARN errors.
	rp := NewRoleMapper(roleKey, externalIDKey, "", false, namespaceKey, &iam.Client{BaseARN: ""}, store, "glob")
	_, err := rp.GetRoleMapping("10.0.0.1")
	if err == nil {
		t.Error("expected error when no annotation and no default role, got nil")
	}
}

func TestGetRoleMappingWithDefault(t *testing.T) {
	pod := &v1.Pod{}
	pod.Status.PodIP = "10.0.0.2"
	store := &storeMock{pods: map[string]*v1.Pod{"10.0.0.2": pod}}

	const defaultRole = "default-role"
	rp := NewRoleMapper(roleKey, externalIDKey, defaultRole, false, namespaceKey, &iam.Client{BaseARN: defaultBaseRole}, store, "glob")
	result, err := rp.GetRoleMapping("10.0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedARN := defaultBaseRole + defaultRole
	if result.Role != expectedARN {
		t.Errorf("expected role %q, got %q", expectedARN, result.Role)
	}
}

func TestGetRoleMappingPodNotFound(t *testing.T) {
	store := &storeMock{podErr: fmt.Errorf("pod not found")}
	rp := NewRoleMapper(roleKey, externalIDKey, "", false, namespaceKey, &iam.Client{BaseARN: defaultBaseRole}, store, "glob")
	_, err := rp.GetRoleMapping("10.99.99.99")
	if err == nil {
		t.Error("expected error when pod not found, got nil")
	}
}

// ---- GetExternalIDMapping tests ---------------------------------------------

func TestGetExternalIDMappingWithAnnotation(t *testing.T) {
	const externalID = "my-external-id"
	pod := &v1.Pod{}
	pod.Status.PodIP = "10.0.0.3"
	pod.Annotations = map[string]string{externalIDKey: externalID}
	store := &storeMock{pods: map[string]*v1.Pod{"10.0.0.3": pod}}

	rp := NewRoleMapper(roleKey, externalIDKey, "", false, namespaceKey, &iam.Client{BaseARN: defaultBaseRole}, store, "glob")
	got, err := rp.GetExternalIDMapping("10.0.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != externalID {
		t.Errorf("expected external ID %q, got %q", externalID, got)
	}
}

func TestGetExternalIDMappingWithoutAnnotation(t *testing.T) {
	pod := &v1.Pod{}
	pod.Status.PodIP = "10.0.0.4"
	store := &storeMock{pods: map[string]*v1.Pod{"10.0.0.4": pod}}

	rp := NewRoleMapper(roleKey, externalIDKey, "", false, namespaceKey, &iam.Client{BaseARN: defaultBaseRole}, store, "glob")
	got, err := rp.GetExternalIDMapping("10.0.0.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty external ID when annotation absent, got %q", got)
	}
}

// ---- DumpDebugInfo tests ----------------------------------------------------

func TestDumpDebugInfo(t *testing.T) {
	pod := &v1.Pod{}
	pod.Status.PodIP = "10.0.0.5"
	pod.Annotations = map[string]string{roleKey: "debug-role"}
	pod.Namespace = "default"

	ns := &v1.Namespace{}
	ns.Name = "default"
	ns.Annotations = map[string]string{namespaceKey: `["debug-role"]`}

	store := &storeMock{
		pods:   map[string]*v1.Pod{"10.0.0.5": pod},
		nsList: []string{"default"},
		nsMap:  map[string]*v1.Namespace{"default": ns},
	}

	rp := NewRoleMapper(roleKey, externalIDKey, "", false, namespaceKey, &iam.Client{BaseARN: defaultBaseRole}, store, "glob")
	result := rp.DumpDebugInfo()

	if _, ok := result["rolesByIP"]; !ok {
		t.Error("expected 'rolesByIP' key in DumpDebugInfo output")
	}
	if _, ok := result["namespaceByIP"]; !ok {
		t.Error("expected 'namespaceByIP' key in DumpDebugInfo output")
	}
	if _, ok := result["rolesByNamespace"]; !ok {
		t.Error("expected 'rolesByNamespace' key in DumpDebugInfo output")
	}

	rolesByIP := result["rolesByIP"].(map[string]string)
	if rolesByIP["10.0.0.5"] != "debug-role" {
		t.Errorf("expected role 'debug-role' for IP 10.0.0.5, got %q", rolesByIP["10.0.0.5"])
	}
}
