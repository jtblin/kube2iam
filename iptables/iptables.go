package iptables

import (
	"errors"
	"net"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

// AddRule adds the required rule to the host's nat table.
func AddRule(appPort, metadataAddress, hostInterface, hostIP string) error {

	if err := checkInterfaceExists(hostInterface); err != nil {
		return err
	}

	if hostIP == "" {
		return errors.New("--host-ip must be set")
	}

	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	ruleSpec := []string{"-p", "tcp", "-d", metadataAddress, "--dport", "80",
		"-j", "DNAT", "--to-destination", hostIP + ":" + appPort,
	}
	if strings.HasPrefix(hostInterface, "!") {
		ruleSpec = append(ruleSpec, "!")
	}
	ruleSpec = append(ruleSpec, "-i", strings.TrimPrefix(hostInterface, "!"))
	return ipt.AppendUnique(
		"nat", "PREROUTING", ruleSpec...,
	)
}

// checkInterfaceExists validates the interface passed exists for the given system.
// checkInterfaceExists ignores wildcard networks.
func checkInterfaceExists(hostInterface string) error {

	if strings.Contains(hostInterface, "+") {
		// wildcard networks ignored
		return nil
	}

	if strings.HasPrefix(hostInterface, "!") {
		hostInterface = strings.TrimPrefix(hostInterface, "!")
	}

	_, err := net.InterfaceByName(hostInterface)
	return err
}
