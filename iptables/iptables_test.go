package iptables

import (
	"runtime"
	"testing"
)

func TestInterfaceExistsFailWithBogusInterface(t *testing.T) {
	ifc := "bogus0"
	if err := CheckInterfaceExists(ifc); err == nil {
		t.Error("Should fail with interface '%s'", ifc)
	}
}

func TestInterfaceExistsPassWithValidInterface(t *testing.T) {
	var ifc string
	switch os := runtime.GOOS; os {
	case "darwin":
		ifc = "lo0"
	case "linux":
		ifc = "lo"
	default:
		// everything else that we don't know or care about...fail
		ifc = "unknown"
		t.Error("%s OS '%s'\n", ifc, os)
	}
	if err := CheckInterfaceExists(ifc); err != nil {
		t.Error("Should pass with interface '%s'", ifc)
	}
}

func TestAddRule(t *testing.T) {
	t.Skip()
}
