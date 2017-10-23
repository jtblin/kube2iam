package main

import (
	"testing"
)

func TestParseRoleAliasConfigMapsFlag(t *testing.T) {
	ret := parseRoleAliasConfigMapsFlag([]string{"default:role-alias1", "default:role-alias2", "kube-system:role-alias"})
	if len(ret) != 2 {
		t.Fatalf("Unexpected number of total parsed role alias config maps: %d", len(ret))
	}

	if len(ret["default"]) != 2 {
		t.Fatalf("Unexpected number of parsed role alias for config map default: %d", len(ret["default"]))
	}

	if len(ret["kube-system"]) != 1 {
		t.Fatalf("Unexpected number of parsed role alias for config map kube-system: %d", len(ret["kube-system"]))
	}
}
