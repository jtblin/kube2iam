package kube2iam

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// ---- GetNamespaceRoleAnnotation ---------------------------------------------

func TestGetNamespaceRoleAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		wantLen    int
		wantFirst  string
	}{
		{
			name:       "empty annotation returns nil",
			annotation: "",
			wantLen:    0,
		},
		{
			name:       "malformed JSON returns nil",
			annotation: "not valid json",
			wantLen:    0,
		},
		{
			name:       "single role",
			annotation: `["my-role"]`,
			wantLen:    1,
			wantFirst:  "my-role",
		},
		{
			name:       "multiple roles",
			annotation: `["role-a","role-b","role-c"]`,
			wantLen:    3,
			wantFirst:  "role-a",
		},
		{
			name:       "empty JSON array",
			annotation: `[]`,
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &v1.Namespace{}
			ns.Annotations = map[string]string{"namespaceKey": tt.annotation}
			result := GetNamespaceRoleAnnotation(ns, "namespaceKey")

			if len(result) != tt.wantLen {
				t.Errorf("expected %d roles, got %d: %v", tt.wantLen, len(result), result)
			}
			if tt.wantFirst != "" && len(result) > 0 && result[0] != tt.wantFirst {
				t.Errorf("expected first role %q, got %q", tt.wantFirst, result[0])
			}
		})
	}
}

func TestGetNamespaceRoleAnnotationMissingKey(t *testing.T) {
	ns := &v1.Namespace{}
	ns.Annotations = map[string]string{"other-key": `["role"]`}
	result := GetNamespaceRoleAnnotation(ns, "namespaceKey")
	if result != nil {
		t.Errorf("expected nil for missing annotation key, got %v", result)
	}
}

func TestGetNamespaceRoleAnnotationNilAnnotations(t *testing.T) {
	ns := &v1.Namespace{}
	// No annotations set at all
	result := GetNamespaceRoleAnnotation(ns, "namespaceKey")
	if result != nil {
		t.Errorf("expected nil for nil annotations, got %v", result)
	}
}

// ---- NamespaceIndexFunc -----------------------------------------------------

func TestNamespaceIndexFunc(t *testing.T) {
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-namespace"}}
	keys, err := NamespaceIndexFunc(ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "my-namespace" {
		t.Errorf("expected [\"my-namespace\"], got %v", keys)
	}
}

func TestNamespaceIndexFuncWrongType(t *testing.T) {
	_, err := NamespaceIndexFunc("not-a-namespace")
	if err == nil {
		t.Error("expected error for wrong type, got nil")
	}
}

// ---- NamespaceHandler events ------------------------------------------------

func TestNamespaceHandlerOnAdd(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:        "test",
		Annotations: map[string]string{"ns-key": `["my-role"]`},
	}}
	// Should not panic
	h.OnAdd(ns, false)
}

func TestNamespaceHandlerOnAddWrongType(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	// Should not panic; logs an error
	h.OnAdd("not-a-namespace", false)
}

func TestNamespaceHandlerOnUpdate(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	h.OnUpdate(ns, ns)
}

func TestNamespaceHandlerOnUpdateWrongType(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	h.OnUpdate("old", "new")
}

func TestNamespaceHandlerOnDelete(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	h.OnDelete(ns)
}

func TestNamespaceHandlerOnDeleteWrongType(t *testing.T) {
	h := NewNamespaceHandler("ns-key")
	h.OnDelete("not-a-namespace")
}

// ---- isPodActive ------------------------------------------------------------

func TestIsPodActive(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name: "running with IP is active",
			pod: &v1.Pod{Status: v1.PodStatus{
				PodIP: "10.0.0.1",
				Phase: v1.PodRunning,
			}},
			expected: true,
		},
		{
			name: "pending with IP is active",
			pod: &v1.Pod{Status: v1.PodStatus{
				PodIP: "10.0.0.2",
				Phase: v1.PodPending,
			}},
			expected: true,
		},
		{
			name: "succeeded is not active",
			pod: &v1.Pod{Status: v1.PodStatus{
				PodIP: "10.0.0.3",
				Phase: v1.PodSucceeded,
			}},
			expected: false,
		},
		{
			name: "failed is not active",
			pod: &v1.Pod{Status: v1.PodStatus{
				PodIP: "10.0.0.4",
				Phase: v1.PodFailed,
			}},
			expected: false,
		},
		{
			name: "running without IP is not active",
			pod: &v1.Pod{Status: v1.PodStatus{
				PodIP: "",
				Phase: v1.PodRunning,
			}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPodActive(tt.pod)
			if got != tt.expected {
				t.Errorf("isPodActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ---- PodIPIndexFunc ---------------------------------------------------------

func TestPodIPIndexFuncActive(t *testing.T) {
	pod := &v1.Pod{Status: v1.PodStatus{PodIP: "10.0.0.1", Phase: v1.PodRunning}}
	keys, err := PodIPIndexFunc(pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "10.0.0.1" {
		t.Errorf("expected [\"10.0.0.1\"], got %v", keys)
	}
}

func TestPodIPIndexFuncInactive(t *testing.T) {
	pod := &v1.Pod{Status: v1.PodStatus{PodIP: "10.0.0.1", Phase: v1.PodSucceeded}}
	keys, err := PodIPIndexFunc(pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty keys for inactive pod, got %v", keys)
	}
}

func TestPodIPIndexFuncWrongType(t *testing.T) {
	_, err := PodIPIndexFunc("not-a-pod")
	if err == nil {
		t.Error("expected error for wrong type, got nil")
	}
}

// ---- PodHandler events ------------------------------------------------------

func TestPodHandlerOnAdd(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"iam.amazonaws.com/role": "my-role",
			},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.1", Phase: v1.PodRunning},
	}
	h.OnAdd(pod, false)
}

func TestPodHandlerOnAddWrongType(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	h.OnAdd("not-a-pod", false)
}

func TestPodHandlerOnUpdate(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}
	h.OnUpdate(pod, pod)
}

func TestPodHandlerOnUpdateWrongType(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	h.OnUpdate("old", "new")
}

func TestPodHandlerOnDelete(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}
	h.OnDelete(pod)
}

func TestPodHandlerOnDeleteTombstone(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "tombstoned-pod"}}
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/tombstoned-pod",
		Obj: pod,
	}
	// Should not panic
	h.OnDelete(tombstone)
}

func TestPodHandlerOnDeleteWrongType(t *testing.T) {
	h := NewPodHandler("iam.amazonaws.com/role")
	h.OnDelete("not-a-pod")
}
