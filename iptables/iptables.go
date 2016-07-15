package iptables

import (
	"errors"
	"os"

	"github.com/coreos/go-iptables/iptables"
)

// AddRule adds the required rule to the host's nat table
func AddRule(appPort, metadataAddress, hostInterface string) error {
	hostIP := os.Getenv("HOST_IP")
	if hostIP == "" {
		return errors.New("HOST_IP environment variable must be set")
	}

	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	if err := ipt.AppendUnique(
		"nat", "PREROUTING", "-p", "tcp", "-d", metadataAddress, "--dport", "80",
		"-j", "DNAT", "--to-destination", hostIP+":"+appPort, "-i", hostInterface,
	); err != nil {
		return err
	}

	return nil
}
