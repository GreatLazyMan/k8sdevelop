//go:build !windows
// +build !windows

package network

//-A POSTROUTING -m comment --comment "simplecni masq" -j SIMPLECNI-POSTRTG
//-A SIMPLECNI-POSTRTG -m comment --comment "simplecni masq" -j RETURN
//-A SIMPLECNI-POSTRTG -s 10.244.0.0/24 -d 10.244.0.0/16 -m comment --comment "simplecni masq" -j RETURN
//-A SIMPLECNI-POSTRTG -s 10.244.0.0/16 -d 10.244.0.0/24 -m comment --comment "simplecni masq" -j RETURN
//-A SIMPLECNI-POSTRTG ! -s 10.244.0.0/16 -d 10.244.0.0/24 -m comment --comment "simplecni masq" -j RETURN
//-A SIMPLECNI-POSTRTG -s 10.244.0.0/16 ! -d 224.0.0.0/4 -m comment --comment "simplecni masq" -j MASQUERADE --random-fully
//-A SIMPLECNI-POSTRTG ! -s 10.244.0.0/16 -d 10.244.0.0/16 -m comment --comment "simplecni masq" -j MASQUERADE --random-fully

// Copyright 2015 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync"

	"github.com/coreos/go-iptables/iptables"
	log "k8s.io/klog/v2"
)

const (
	ipTablesRestoreCmd  string = "iptables-restore"
	ip6TablesRestoreCmd string = "ip6tables-restore"
	ipTablesCmd         string = "iptables"
	ip6TablesCmd        string = "ip6tables"
	KubeProxyMark       string = "0x4000/0x4000"
)

type IPTablesRule struct {
	Table    string
	Action   string
	Chain    string
	Rulespec []string
}

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	ChainExists(table, chain string) (bool, error)
	ClearChain(table, chain string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

func ipTablesRulesExist(ipt IPTables, rules []IPTablesRule) (bool, error) {
	for _, rule := range rules {
		if rule.Chain == "SIMPLECNI-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-FWD" {
			chainExist, err := ipt.ChainExists(rule.Table, "SIMPLECNI-FWD")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		} else if rule.Chain == "SIMPLECNI-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.Table, "SIMPLECNI-POSTRTG")
			if err != nil {
				return false, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				return false, nil
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return false, fmt.Errorf("failed to check rule existence: %v", err)
		}
		if !exists {
			return false, nil
		}
	}

	return true, nil
}

// ipTablesCleanAndBuild create from a list of iptables rules a transaction (as string) for iptables-restore for ordering the rules that effectively running
func ipTablesCleanAndBuild(ipt IPTables, rules []IPTablesRule) (IPTablesRestoreRules, error) {
	tablesRules := IPTablesRestoreRules{}

	// Build append and delete rules
	for _, rule := range rules {
		if rule.Chain == "SIMPLECNI-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-FWD" {
			chainExist, err := ipt.ChainExists(rule.Table, "SIMPLECNI-FWD")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.Table, "SIMPLECNI-FWD")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		} else if rule.Chain == "SIMPLECNI-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-POSTRTG" {
			chainExist, err := ipt.ChainExists(rule.Table, "SIMPLECNI-POSTRTG")
			if err != nil {
				return nil, fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExist {
				err = ipt.ClearChain(rule.Table, "SIMPLECNI-POSTRTG")
				if err != nil {
					return nil, fmt.Errorf("failed to create rule chain: %v", err)
				}
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return nil, fmt.Errorf("failed to check rule existence: %v", err)
		}
		if exists {
			if _, ok := tablesRules[rule.Table]; !ok {
				tablesRules[rule.Table] = []IPTablesRestoreRuleSpec{}
			}
			// if the rule exists it's safer to delete it and then create them
			tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{"-D", rule.Chain}, rule.Rulespec...))
		}
		// with iptables-restore we can ensure that all rules created are in good order and have no external rule between them
		tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{rule.Action, rule.Chain}, rule.Rulespec...))
	}

	return tablesRules, nil
}

// IpTablesBootstrap init iptables rules using iptables-restore (with some cleaning if some rules already exists)
func IpTablesBootstrap(ipt IPTables, iptRestore IPTablesRestore, rules []IPTablesRule) error {
	tablesRules, err := ipTablesCleanAndBuild(ipt, rules)
	if err != nil {
		// if we can't find iptables or if we can check existing rules, give up and return
		return fmt.Errorf("failed to setup iptables-restore payload: %v", err)
	}

	log.V(6).Infof("trying to run iptables-restore < %+v", tablesRules)

	err = iptRestore.ApplyWithoutFlush(tablesRules)
	if err != nil {
		return fmt.Errorf("failed to apply partial iptables-restore %v", err)
	}

	log.Infof("bootstrap done")

	return nil
}

func EnsureIPTables(ipt IPTables, iptRestore IPTablesRestore, rules []IPTablesRule) error {
	exists, err := ipTablesRulesExist(ipt, rules)
	if err != nil {
		return fmt.Errorf("error checking rule existence: %v", err)
	}
	if exists {
		// if all the rules already exist, no need to do anything
		return nil
	}
	// Otherwise, teardown all the rules and set them up again
	// We do this because the order of the rules is important
	log.Info("Some iptables rules are missing; deleting and recreating rules")
	err = IpTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		return fmt.Errorf("error setting up rules: %v", err)
	}
	return nil
}

func TeardownIPTables(ipt IPTables, iptr IPTablesRestore, rules []IPTablesRule) error {
	tablesRules := IPTablesRestoreRules{}

	// Build delete rules to a transaction for iptables restore
	for _, rule := range rules {
		if rule.Chain == "SIMPLECNI-FWD" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-FWD" {
			chainExists, err := ipt.ChainExists(rule.Table, "SIMPLECNI-FWD")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		} else if rule.Chain == "SIMPLECNI-POSTRTG" || rule.Rulespec[len(rule.Rulespec)-1] == "SIMPLECNI-POSTRTG" {
			chainExists, err := ipt.ChainExists(rule.Table, "SIMPLECNI-POSTRTG")
			if err != nil {
				// this shouldn't ever happen
				return fmt.Errorf("failed to check rule existence: %v", err)
			}
			if !chainExists {
				continue
			}
		}
		exists, err := ipt.Exists(rule.Table, rule.Chain, rule.Rulespec...)
		if err != nil {
			// this shouldn't ever happen
			return fmt.Errorf("failed to check rule existence: %v", err)
		}

		if exists {
			if _, ok := tablesRules[rule.Table]; !ok {
				tablesRules[rule.Table] = []IPTablesRestoreRuleSpec{}
			}
			tablesRules[rule.Table] = append(tablesRules[rule.Table], append(IPTablesRestoreRuleSpec{"-D", rule.Chain}, rule.Rulespec...))
		}
	}
	err := iptr.ApplyWithoutFlush(tablesRules) // ApplyWithoutFlush make a diff, Apply make a replace (desired state)
	if err != nil {
		return fmt.Errorf("unable to teardown iptables: %v", err)
	}

	return nil
}

// IPTablesRestore wrapper for iptables-restore
type IPTablesRestore interface {
	// ApplyWithoutFlush apply without flush chains
	ApplyWithoutFlush(rules IPTablesRestoreRules) error
}

// ipTablesRestore internal type
type ipTablesRestore struct {
	path    string
	proto   iptables.Protocol
	hasWait bool
	// ipTablesRestore needs a mutex to ensure that two avoid
	// collisions between two goroutines calling ApplyWithoutFlush in parallel.
	// This could result in the second call accidentally restoring a rule removed by the first
	mu sync.Mutex
}

// IPTablesRestoreRules represents iptables-restore table block
type IPTablesRestoreRules map[string][]IPTablesRestoreRuleSpec

// IPTablesRestoreRuleSpec represents one rule spec delimited by space
type IPTablesRestoreRuleSpec []string

// NewIPTablesRestoreWithProtocol build new IPTablesRestore for supplied proto
func NewIPTablesRestoreWithProtocol(protocol iptables.Protocol) (IPTablesRestore, error) {
	cmd := getIptablesRestoreCommand(protocol)
	path, err := exec.LookPath(cmd)
	if err != nil {
		return nil, err
	}
	cmdIptables := getIptablesCommand(protocol)
	pathIptables, err := exec.LookPath(cmdIptables)
	if err != nil {
		return nil, err
	}
	hasWait, err := getIptablesRestoreSupport(pathIptables)
	if err != nil {
		return nil, err
	}

	ipt := ipTablesRestore{
		path:    path,
		proto:   protocol,
		hasWait: hasWait,
		mu:      sync.Mutex{},
	}
	return &ipt, nil
}

// ApplyWithoutFlush apply without flush chains
func (iptr *ipTablesRestore) ApplyWithoutFlush(rules IPTablesRestoreRules) error {
	iptr.mu.Lock()
	defer iptr.mu.Unlock()
	payload := buildIPTablesRestorePayload(rules)

	log.V(6).Infof("trying to run with payload %s", payload)
	stdout, stderr, err := iptr.runWithOutput([]string{"--noflush"}, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return fmt.Errorf("unable to run iptables-restore (%s, %s): %v", stdout, stderr, err)
	}
	return nil
}

// runWithOutput runs an iptables command with the given arguments,
// writing any stdout output to the given writer
func (iptr *ipTablesRestore) runWithOutput(args []string, stdin io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if iptr.hasWait {
		args = append(args, "--wait")
	}

	cmd := exec.Command(iptr.path, args...)
	log.Infof("command is %s,args is %v", iptr.path, args)
	var buf bytes.Buffer
	teeReader := io.TeeReader(stdin, &buf)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = teeReader

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}

	dataStdin, _ := io.ReadAll(&buf)
	log.Infof("command stdin is %s", string(dataStdin))
	log.Infof("command stdin is %v %d", dataStdin, len(dataStdin))
	return stdout.String(), stderr.String(), nil
}

// buildIPTablesRestorePayload build table/COMMIT payload
func buildIPTablesRestorePayload(tableRules IPTablesRestoreRules) string {
	iptablesRestorePayload := ""
	for table, rules := range tableRules {
		iptablesRestorePayload += "*" + table + "\n"

		for _, lineRule := range rules {
			// as iptables-restore use stdin if "--comment" then protect "the comment"
			size := len(lineRule)
			for i, rule := range lineRule {
				if i > 0 && lineRule[i-1] == "--comment" {
					iptablesRestorePayload += "\"" + rule + "\""
				} else {
					iptablesRestorePayload += rule
				}

				if i < size-1 {
					iptablesRestorePayload += " "
				} else {
					iptablesRestorePayload += "\n"
				}
			}
		}

		iptablesRestorePayload += "COMMIT\n"
	}
	return iptablesRestorePayload
}

// getIptablesRestoreSupport get current iptables-restore support
func getIptablesRestoreSupport(path string) (hasWait bool, err error) {
	version, err := getIptablesRestoreVersionString(path)
	if err != nil {
		return false, err
	}
	v1, v2, v3, err := extractIptablesRestoreVersion(version)
	if err != nil {
		return false, err
	}
	return ipTablesHasWaitSupport(v1, v2, v3), nil
}

// Checks if an iptables-restore version is after 1.6.2, when --wait was added
func ipTablesHasWaitSupport(v1, v2, v3 int) bool {
	if v1 > 1 {
		return true
	}
	if v1 == 1 && v2 > 6 {
		return true
	}
	if v1 == 1 && v2 == 6 && v3 >= 2 {
		return true
	}
	return false
}

// extractIptablesRestoreVersion returns the first three components of the iptables-restore version
// e.g. "iptables-restore v1.3.66" would return (1, 3, 66, nil)
func extractIptablesRestoreVersion(str string) (int, int, int, error) {
	versionMatcher := regexp.MustCompile(`v([0-9]+)\.([0-9]+)\.([0-9]+)`)
	result := versionMatcher.FindStringSubmatch(str)
	if result == nil {
		return 0, 0, 0, fmt.Errorf("no iptables-restore version found in string: %s", str)
	}

	v1, err := strconv.Atoi(result[1])
	if err != nil {
		return 0, 0, 0, err
	}

	v2, err := strconv.Atoi(result[2])
	if err != nil {
		return 0, 0, 0, err
	}

	v3, err := strconv.Atoi(result[3])
	if err != nil {
		return 0, 0, 0, err
	}
	return v1, v2, v3, nil
}

// getIptablesRestoreVersionString run iptables-restore --version
func getIptablesRestoreVersionString(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("unable to find iptables-restore version: %v", err)
	}
	return out.String(), nil
}

// getIptablesRestoreCommand returns the correct command for the given proto, either "iptables-restore" or "ip6tables-restore".
func getIptablesRestoreCommand(proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return ip6TablesRestoreCmd
	}
	return ipTablesRestoreCmd
}

// getIptablesCommand returns the correct command for the given proto, either "iptables" or "ip6tables".
func getIptablesCommand(proto iptables.Protocol) string {
	if proto == iptables.ProtocolIPv6 {
		return ip6TablesCmd
	}
	return ipTablesCmd
}
