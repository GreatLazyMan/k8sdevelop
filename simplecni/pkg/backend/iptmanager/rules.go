package iptmanager

import (
	"net"

	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"github.com/coreos/go-iptables/iptables"
)

func masqIP6Rules(ccidr, pcidr *net.IPNet) []network.IPTablesRule {
	cluster_cidr := ccidr.String()
	pod_cidr := pcidr.String()
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	supports_random_fully := false
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}
	rules := make([]network.IPTablesRule, 2)

	// This rule ensure that the flannel iptables rules are executed before other rules on the node
	rules[0] = network.IPTablesRule{Table: "nat", Action: "-A", Chain: "POSTROUTING", Rulespec: []string{"-m", "comment", "--comment", "simplecni masq", "-j", "SIMPLECNI-POSTRTG"}}
	// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
	rules[1] = network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"-m", "mark", "--mark", KubeProxyMark, "-m", "comment", "--comment", "simplecni masq", "-j", "RETURN"}}

	// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0), for any of the cluster_cidrs
	rules = append(rules,
		network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"-s", pod_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "RETURN"}},
		network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "RETURN"}},
	)
	// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
	rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "RETURN"}})
	// NAT if it's not multicast traffic
	if supports_random_fully {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE", "--random-fully"}})
	} else {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"-s", cluster_cidr, "!", "-d", "ff00::/8", "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE"}})
	}

	// Masquerade anything headed towards flannel from the host
	if supports_random_fully {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE", "--random-fully"}})
	} else {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG", Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE"}})
	}

	return rules
}

func forwardRules(ccidr *net.IPNet) []network.IPTablesRule {
	cluster_cidr := ccidr.String()
	return []network.IPTablesRule{
		// This rule ensure that the flannel iptables rules are executed before other rules on the node
		{Table: "filter", Action: "-A", Chain: "FORWARD", Rulespec: []string{"-m", "comment", "--comment", "simplecni forward", "-j", "SIMPLECNI-FWD"}},
		// These rules allow traffic to be forwarded if it is to or from the flannel network range.
		{Table: "filter", Action: "-A", Chain: "SIMPLECNI-FWD", Rulespec: []string{"-s", cluster_cidr, "-m", "comment", "--comment", "simplecni forward", "-j", "ACCEPT"}},
		{Table: "filter", Action: "-A", Chain: "SIMPLECNI-FWD", Rulespec: []string{"-d", cluster_cidr, "-m", "comment", "--comment", "simplecni forward", "-j", "ACCEPT"}},
	}
}

func masqRules(ccidr, pcidr *net.IPNet) []network.IPTablesRule {
	cluster_cidr := ccidr.String()
	pod_cidr := pcidr.String()
	ipt, err := iptables.New()
	supports_random_fully := false
	if err == nil {
		supports_random_fully = ipt.HasRandomFully()
	}
	rules := make([]network.IPTablesRule, 2)
	// This rule ensure that the flannel iptables rules are executed before other rules on the node
	rules[0] = network.IPTablesRule{Table: "nat", Action: "-A", Chain: "POSTROUTING",
		Rulespec: []string{"-m", "comment", "--comment", "simplecni masq", "-j", "SIMPLECNI-POSTRTG"}}
	// This rule will not masquerade traffic marked by the kube-proxy to avoid double NAT bug on some kernel version
	rules[1] = network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
		Rulespec: []string{"-m", "mark", "--mark", KubeProxyMark, "-m", "comment", "--comment", "simplecni masq mask", "-j", "RETURN"}}
	// This rule makes sure we don't NAT traffic within overlay network (e.g. coming out of docker0), for any of the cluster_cidrs
	rules = append(rules,
		network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"-s", pod_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq pod to cluster", "-j", "RETURN"}},
		network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "simplecni masq cluster to pod", "-j", "RETURN"}},
	)
	// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
	rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
		Rulespec: []string{"!", "-s", cluster_cidr, "-d", pod_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "RETURN"}})
	// NAT if it's not multicast traffic
	if supports_random_fully {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"-s", cluster_cidr, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE", "--random-fully"}})
	} else {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"-s", cluster_cidr, "!", "-d", "224.0.0.0/4", "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE"}})
	}
	// Masquerade anything headed towards flannel from the host
	if supports_random_fully {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE", "--random-fully"}})
	} else {
		rules = append(rules, network.IPTablesRule{Table: "nat", Action: "-A", Chain: "SIMPLECNI-POSTRTG",
			Rulespec: []string{"!", "-s", cluster_cidr, "-d", cluster_cidr, "-m", "comment", "--comment", "simplecni masq", "-j", "MASQUERADE"}})
	}
	return rules
}
