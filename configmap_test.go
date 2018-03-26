package kube2iam

import (
	"reflect"
	"testing"

	"k8s.io/client-go/pkg/api/v1"
)

func TestConfigMapIndexFunc(t *testing.T) {
	f := ConfigMapIndexFunc(map[string][]string{"kube-system": []string{"role-alias"}})
	s, err := f(&v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "role-alias",
			Namespace: "kube-system",
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error %s", err)
	}

	if !reflect.DeepEqual(s, []string{"role-alias"}) {
		t.Fatalf("Unexpected index names: %v", s)
	}
}
