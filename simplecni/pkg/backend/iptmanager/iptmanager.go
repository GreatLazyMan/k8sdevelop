package iptmanager

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
	"context"
	"fmt"
	"net"
	"time"

	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
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

type IPTablesManager struct {
	ipv4Rules    []network.IPTablesRule
	ipv6Rules    []network.IPTablesRule
	forwardRules []network.IPTablesRule
	*CidrManager
}

func (iptm *IPTablesManager) Init(ctx context.Context, ccidr, ncidr, ccidrv6, ncidrv6 *net.IPNet) error {
	log.Info("init iptables manager")
	iptm.forwardRules = forwardRules(ccidr)
	iptm.ipv4Rules = masqRules(ccidr, ncidr)
	iptm.ipv6Rules = masqIP6Rules(ccidrv6, ncidrv6)

	cidrManager, err := NewCidrManager(ccidr, ncidr, ccidrv6, ncidrv6)
	if err != nil {
		return err
	}
	iptm.CidrManager = cidrManager

	iptm.SetupAndEnsureMasqRules(ctx, 1)
	iptm.SetupAndEnsureForwardRules(ctx, 1)
	go func() {
		<-ctx.Done()
		time.Sleep(time.Second)
		err := iptm.CleanUp()
		if err != nil {
			log.Errorf("iptables: error while cleaning-up: %v", err)
		}
	}()

	return nil
}

func (iptm *IPTablesManager) CleanUp() error {
	if len(iptm.ipv4Rules) > 0 {
		ipt, err := iptables.New()
		if err != nil {
			// if we can't find iptables, give up and return
			return fmt.Errorf("failed to setup IPTables. iptables binary was not found: %v", err)
		}
		iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv4)
		if err != nil {
			// if we can't find iptables-restore, give up and return
			return fmt.Errorf("failed to setup IPTables. iptables-restore binary was not found: %v", err)
		}
		log.Info("iptables (ipv4): cleaning-up before exiting flannel...")
		err = network.TeardownIPTables(ipt, iptRestore, iptm.ipv4Rules)
		if err != nil {
			log.Errorf("Failed to tear down IPTables: %v", err)
		}
	}
	if len(iptm.ipv6Rules) > 0 {
		ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
		if err != nil {
			// if we can't find iptables, give up and return
			return fmt.Errorf("failed to setup IPTables. iptables binary was not found: %v", err)
		}
		iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv6)
		if err != nil {
			// if we can't find iptables-restore, give up and return
			return fmt.Errorf("failed to setup IPTables. iptables-restore binary was not found: %v", err)
		}
		log.Info("iptables (ipv6): cleaning-up before exiting flannel...")
		err = network.TeardownIPTables(ipt, iptRestore, iptm.ipv6Rules)
		if err != nil {
			log.Errorf("Failed to tear down IPTables: %v", err)
		}
	}
	return nil
}

func (iptm *IPTablesManager) SetupAndEnsureMasqRules(ctx context.Context, resyncPeriod int) error {

	// recycle iptables rules only when network configured or subnet leased is not equal to current one.
	if iptm.prevnodecidr.String() != iptm.nodecidr.String() || iptm.prevclustercidr.String() != iptm.clustercidr.String() {
		log.Infof("Current network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old iptables rules",
			iptm.nodecidr, iptm.clustercidr, iptm.prevnodecidr, iptm.prevclustercidr)
		if err := iptm.deleteIP4Tables(masqRules(iptm.prevclustercidr, iptm.prevnodecidr)); err != nil {
			return err
		}
	}

	log.Infof("Setting up masking rules")
	iptm.CreateIP4Chain("nat", "SIMPLECNI-POSTRTG")
	go iptm.setupAndEnsureIP4Tables(ctx, masqRules(iptm.clustercidr, iptm.nodecidr), resyncPeriod)

	// recycle iptables rules only when network configured or subnet leased is not equal to current one.
	if iptm.nodecidrv6 != nil && iptm.clustercidrv6 != nil {
		if iptm.prevnodecidrv6.String() != iptm.nodecidrv6.String() || iptm.prevclustercidrv6.String() != iptm.clustercidrv6.String() {
			log.Infof("Current network or subnet (%v, %v) is not equal to previous one (%v, %v), trying to recycle old iptables rules",
				iptm.nodecidrv6, iptm.clustercidrv6, iptm.prevnodecidrv6, iptm.prevclustercidrv6)
			if err := iptm.deleteIP6Tables(masqIP6Rules(iptm.clustercidrv6, iptm.nodecidrv6)); err != nil {
				return err
			}
		}
	}

	if iptm.nodecidrv6 != nil && iptm.clustercidrv6 != nil {
		log.Infof("Setting up masking rules for IPv6")
		iptm.CreateIP6Chain("nat", "SIMPLECNI-POSTRTG")
		go iptm.setupAndEnsureIP6Tables(ctx, masqIP6Rules(iptm.clustercidrv6, iptm.nodecidrv6), resyncPeriod)
	}
	return nil
}

func (iptm *IPTablesManager) SetupAndEnsureForwardRules(ctx context.Context, resyncPeriod int) {
	if iptm.clustercidr != nil {
		log.Infof("Changing default FORWARD chain policy to ACCEPT")
		iptm.CreateIP4Chain("filter", "SIMPLECNI-FWD")
		go iptm.setupAndEnsureIP4Tables(ctx, forwardRules(iptm.nodecidr), resyncPeriod)
	}
	if iptm.clustercidrv6 != nil {
		log.Infof("IPv6: Changing default FORWARD chain policy to ACCEPT")
		iptm.CreateIP6Chain("filter", "SIMPLECNI-FWD")
		go iptm.setupAndEnsureIP6Tables(ctx, forwardRules(iptm.nodecidrv6), resyncPeriod)
	}
}

func (iptm *IPTablesManager) CreateIP4Chain(table, chain string) {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return
	}
	err = ipt.ClearChain(table, chain)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. Error on creating the chain: %v", err)
		return
	}
}

func (iptm *IPTablesManager) CreateIP6Chain(table, chain string) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return
	}
	err = ipt.ClearChain(table, chain)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. Error on creating the chain: %v", err)
		return
	}
}

func (iptm *IPTablesManager) setupAndEnsureIP4Tables(ctx context.Context, rules []network.IPTablesRule, resyncPeriod int) {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return
	}
	iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		// if we can't find iptables-restore, give up and return
		log.Errorf("Failed to setup IPTables. iptables-restore binary was not found: %v", err)
		return
	}

	err = network.IpTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to bootstrap IPTables: %v", err)
	}

	iptm.ipv4Rules = append(iptm.ipv4Rules, rules...)
	for {
		select {
		case <-ctx.Done():
			//clean-up is setup in Init
			return
		case <-time.After(time.Duration(resyncPeriod) * time.Second):
			// Ensure that all the iptables rules exist every 5 seconds
			if err := network.EnsureIPTables(ipt, iptRestore, rules); err != nil {
				log.Errorf("Failed to ensure iptables rules: %v", err)
			}
		}

	}
}

func (iptm *IPTablesManager) setupAndEnsureIP6Tables(ctx context.Context, rules []network.IPTablesRule, resyncPeriod int) {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return
	}
	iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return
	}

	err = network.IpTablesBootstrap(ipt, iptRestore, rules)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to bootstrap IPTables: %v", err)
	}
	iptm.ipv6Rules = append(iptm.ipv6Rules, rules...)

	for {
		select {
		case <-ctx.Done():
			//clean-up is setup in Init
			return
		case <-time.After(time.Duration(resyncPeriod) * time.Second):
			// Ensure that all the iptables rules exist every 5 seconds
			if err := network.EnsureIPTables(ipt, iptRestore, rules); err != nil {
				log.Errorf("Failed to ensure iptables rules: %v", err)
			}
		}
	}
}

// deleteIP4Tables delete specified iptables rules
func (iptm *IPTablesManager) deleteIP4Tables(rules []network.IPTablesRule) error {
	ipt, err := iptables.New()
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IPTables. iptables binary was not found: %v", err)
		return err
	}
	iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return err
	}
	err = network.TeardownIPTables(ipt, iptRestore, rules)
	if err != nil {
		log.Errorf("Failed to teardown iptables: %v", err)
		return err
	}
	return nil
}

// deleteIP6Tables delete specified iptables rules
func (iptm *IPTablesManager) deleteIP6Tables(rules []network.IPTablesRule) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup IP6Tables. iptables binary was not found: %v", err)
		return err
	}

	iptRestore, err := network.NewIPTablesRestoreWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		// if we can't find iptables, give up and return
		log.Errorf("Failed to setup iptables-restore: %v", err)
		return err
	}
	err = network.TeardownIPTables(ipt, iptRestore, rules)
	if err != nil {
		log.Errorf("Failed to teardown iptables: %v", err)
		return err
	}
	return nil
}
