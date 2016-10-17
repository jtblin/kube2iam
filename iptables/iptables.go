package iptables

import (
	"errors"

	"github.com/coreos/go-iptables/iptables"
)

func iptablesInsertUnique(ipt *iptables.IPTables, table, chain string, pos int, rulespec ...string) error {
	exists, err := ipt.Exists(table, chain, rulespec...)
	if err != nil {
		return err
	}

	if exists {
		if err := ipt.Delete(table, chain, rulespec...); err != nil {
			return err
		}
	}

	return ipt.Insert(table, chain, pos, rulespec...)
}

// AddRule adds the required rule to the host's nat table
func AddRule(appPort, metadataAddress, hostInterface, hostIP string) error {
	if hostIP == "" {
		return errors.New("--host-ip must be set")
	}

	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	if err := iptablesInsertUnique(ipt,
		"nat", "PREROUTING", 1, "-p", "tcp", "-d", metadataAddress, "--dport", "80",
		"-j", "DNAT", "--to-destination", hostIP+":"+appPort, "-i", hostInterface,
	); err != nil {
		return err
	}

	return nil
}
