package iptables

import (
	"errors"
	"net"

	"github.com/coreos/go-iptables/iptables"
)

// AddRule adds the required rule to the host's nat table
func AddRule(appPort, metadataAddress, hostInterface, hostIP string) error {

	err := InterfaceExists(hostInterface)
	if err != nil {
		return err
	}

	if hostIP == "" {
		return errors.New("--host-ip must be set")
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

func InterfaceExists(hostInterface string) error {
	_, err := net.InterfaceByName(hostInterface)
	return err
}
