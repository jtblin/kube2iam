package iptables

import (
	"runtime"
	"testing"
)

func TestCheckInterfaceExistsFailsWithBogusInterface(t *testing.T) {
	ifc := "bogus0"
	if err := checkInterfaceExists(ifc); err == nil {
		t.Error("Should fail with invalid interface. Interface received:", ifc)
	}
}

func TestCheckInterfaceExistsPassesWithValidInterface(t *testing.T) {
	var ifc string
	switch os := runtime.GOOS; os {
	case "darwin":
		ifc = "lo0"
	case "linux":
		ifc = "lo"
	default:
		// everything else that we don't know or care about...fail
		ifc = "unknown"
		t.Fatalf("%s OS '%s'\n", ifc, os)
	}
	if err := checkInterfaceExists(ifc); err != nil {
		t.Error("Should pass with valid interface. Interface received:", ifc)
	}
}

func TestCheckInterfaceExistsPassesWithPlus(t *testing.T) {
	ifc := "cali+"
	if err := checkInterfaceExists(ifc); err != nil {
		t.Error("Should pass with external networking. Interface received:", ifc)
	}
}

func TestCheckInterfaceExistsPassesWithNegated(t *testing.T) {
	var ifc string
	switch os := runtime.GOOS; os {
	case "darwin":
		ifc = "!lo0"
	case "linux":
		ifc = "!lo"
	default:
		// everything else that we don't know or care about...fail
		ifc = "!unknown"
		t.Fatalf("%s OS '%s'\n", ifc, os)
	}
	if err := checkInterfaceExists(ifc); err != nil {
		t.Error("Should pass with negated interface(s). Interface received:", ifc)
	}
}

func TestCheckInterfaceExistsFailsWithDoubleNegated(t *testing.T) {
	var ifc string
	switch os := runtime.GOOS; os {
	case "darwin":
		ifc = "!!lo0"
	case "linux":
		ifc = "!!lo"
	default:
		// everything else that we don't know or care about...fail
		ifc = "!!unknown"
		t.Fatalf("%s OS '%s'\n", ifc, os)
	}
	if err := checkInterfaceExists(ifc); err == nil {
		t.Error("Should fail with invalid interface. Interface received:", ifc)
	}
}

func TestAddRule(t *testing.T) {
	t.Skip()
}
