// Copyright 2024 flannel authors
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
//go:build !windows
// +build !windows

package iptmanager

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	log "k8s.io/klog/v2"

	"sigs.k8s.io/knftables"
)

const (
	ipv4Table    = "simplecni-ipv4"
	ipv6Table    = "simplecni-ipv6"
	forwardChain = "forward"
	postrtgChain = "postrtg"
	//maximum delay in second to clean-up when the context is cancelled
	cleanUpDeadline = 15
)

/*
table ip simplecni-ipv4 {
	comment "rules for simplecni-ipv4"
	chain postrtg {
		comment "chain to manage traffic masquerading by simplecni"
		type nat hook postrouting priority srcnat; policy accept;
		meta mark 0x00004000 return
		ip saddr 10.244.0.0/16 ip daddr 10.244.0.0/24 return
		ip saddr 10.244.0.0/24 ip daddr 10.244.0.0/16 return
		ip saddr != 10.244.0.0/16 ip daddr 10.244.0.0/24 return
		ip saddr 10.244.0.0/24 ip daddr != 224.0.0.0/4 masquerade fully-random
		ip saddr != 10.244.0.0/24 ip daddr 10.244.0.0/24 masquerade fully-random
	}

	chain forward {
		comment "chain to accept simplecni traffic"
		type filter hook forward priority filter; policy accept;
		ip saddr 10.244.0.0/24 accept
		ip daddr 10.244.0.0/24 accept
	}
}
table ip6 simplecni-ipv6 {
	comment "rules for simplecni-ipv6"
}
*/

// TODO: copy from flannel, some day rewrite some code
type NFTablesManager struct {
	nftv4 knftables.Interface
	nftv6 knftables.Interface
	*CidrManager
	randomfullySupported bool
}

func (nftm *NFTablesManager) Init(ctx context.Context, ccidr, ncidr, ccidrv6, ncidrv6 *net.IPNet) error {
	log.Info("Starting simplecni in nftables mode...")
	var err error
	nftm.nftv4, err = initTable(ctx, knftables.IPv4Family, ipv4Table)
	if err != nil {
		return err
	}
	nftm.nftv6, err = initTable(ctx, knftables.IPv6Family, ipv6Table)
	if err != nil {
		return err
	}
	cidrManager, err := NewCidrManager(ccidr, ncidr, ccidrv6, ncidrv6)
	if err != nil {
		return err
	}
	nftm.CidrManager = cidrManager
	nftm.randomfullySupported = network.IsRandomfullySupported(ctx, nftm.nftv4)
	nftm.SetupAndEnsureMasqRules(ctx, 1)
	nftm.SetupAndEnsureForwardRules(ctx, 1)

	go func() {
		<-ctx.Done()
		time.Sleep(time.Second)
		err := nftm.CleanUp()
		if err != nil {
			log.Errorf("iptables: error while cleaning-up: %v", err)
		}
	}()

	return nil
}

// create a new table and returns the interface to interact with it
func initTable(ctx context.Context, ipFamily knftables.Family, name string) (knftables.Interface, error) {
	nft, err := knftables.New(ipFamily, name)
	if err != nil {
		return nil, fmt.Errorf("no nftables support: %v", err)
	}
	tx := nft.NewTransaction()

	tx.Add(&knftables.Table{
		Comment: knftables.PtrTo("rules for " + name),
	})
	err = nft.Run(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("nftables: couldn't initialise table %s: %v", name, err)
	}
	return nft, nil
}

// It is needed when using nftables? accept seems to be the default
// warning: never add a default 'drop' policy on the forwardChain as it breaks connectivity to the node
func (nftm *NFTablesManager) SetupAndEnsureForwardRules(ctx context.Context, resyncPeriod int) {
	log.Infof("Changing default FORWARD chain policy to ACCEPT")
	tx := nftm.nftv4.NewTransaction()

	tx.Add(&knftables.Chain{
		Name:     forwardChain,
		Comment:  knftables.PtrTo("chain to accept simplecni traffic"),
		Type:     knftables.PtrTo(knftables.FilterType),
		Hook:     knftables.PtrTo(knftables.ForwardHook),
		Priority: knftables.PtrTo(knftables.FilterPriority),
	})
	tx.Flush(&knftables.Chain{
		Name: forwardChain,
	})

	tx.Add(&knftables.Rule{
		Chain: forwardChain,
		Rule: knftables.Concat(
			"ip saddr", nftm.nodecidr.String(),
			"accept",
		),
	})
	tx.Add(&knftables.Rule{
		Chain: forwardChain,
		Rule: knftables.Concat(
			"ip daddr", nftm.nodecidr.String(),
			"accept",
		),
	})
	err := nftm.nftv4.Run(ctx, tx)
	if err != nil {
		log.Errorf("nftables: couldn't setup forward rules: %v", err)
	}
	if nftm.nodecidr != nil {
		log.Infof("Changing default FORWARD chain policy to ACCEPT (ipv6)")
		tx := nftm.nftv6.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     forwardChain,
			Comment:  knftables.PtrTo("chain to accept simplecni traffic"),
			Type:     knftables.PtrTo(knftables.FilterType),
			Hook:     knftables.PtrTo(knftables.ForwardHook),
			Priority: knftables.PtrTo(knftables.FilterPriority),
		})
		tx.Flush(&knftables.Chain{
			Name: forwardChain,
		})

		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip6 saddr", nftm.nodecidrv6.String(),
				"accept",
			),
		})
		tx.Add(&knftables.Rule{
			Chain: forwardChain,
			Rule: knftables.Concat(
				"ip6 daddr", nftm.nodecidrv6.String(),
				"accept",
			),
		})
		err := nftm.nftv6.Run(ctx, tx)
		if err != nil {
			log.Errorf("nftables: couldn't setup forward rules (ipv6): %v", err)
		}
	}
}

func (nftm *NFTablesManager) SetupAndEnsureMasqRules(ctx context.Context, resyncPeriod int) error {
	if nftm.nodecidr != nil {
		log.Infof("nftables: setting up masking rules (ipv4)")
		tx := nftm.nftv4.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     postrtgChain,
			Comment:  knftables.PtrTo("chain to manage traffic masquerading by simplecni"),
			Type:     knftables.PtrTo(knftables.NATType),
			Hook:     knftables.PtrTo(knftables.PostroutingHook),
			Priority: knftables.PtrTo(knftables.SNATPriority),
		})
		// make sure that the chain is empty before adding our rules
		// => no need for the check and recycle part of iptables.go
		tx.Flush(&knftables.Chain{
			Name: postrtgChain,
		})
		err := nftm.addMasqRules(tx, nftm.nodecidr.String(), nftm.clustercidr.String(), knftables.IPv4Family)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
		err = nftm.nftv4.Run(ctx, tx)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
	}
	if nftm.nodecidrv6 != nil {
		log.Infof("nftables: setting up masking rules (ipv6)")
		tx := nftm.nftv6.NewTransaction()

		tx.Add(&knftables.Chain{
			Name:     postrtgChain,
			Comment:  knftables.PtrTo("chain to manage traffic masquerading by simplecni"),
			Type:     knftables.PtrTo(knftables.NATType),
			Hook:     knftables.PtrTo(knftables.PostroutingHook),
			Priority: knftables.PtrTo(knftables.SNATPriority),
		})
		// make sure that the chain is empty before adding our rules
		// => no need for the check and recycle part of iptables.go
		tx.Flush(&knftables.Chain{
			Name: postrtgChain,
		})
		err := nftm.addMasqRules(tx, nftm.nodecidr.String(), nftm.clustercidr.String(), knftables.IPv6Family)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
		err = nftm.nftv6.Run(ctx, tx)
		if err != nil {
			return fmt.Errorf("nftables: couldn't setup masq rules: %v", err)
		}
	}
	return nil
}

// add required masking rules to transaction tx
func (nftm *NFTablesManager) addMasqRules(
	tx *knftables.Transaction,
	clusterCidr, podCidr string,
	family knftables.Family) error {
	masquerade := "masquerade fully-random"
	if !nftm.randomfullySupported {
		masquerade = "masquerade"
	}

	multicastCidr := "224.0.0.0/4"
	if family == knftables.IPv6Family {
		multicastCidr = "ff00::/8"
	}
	// This rule will not masquerade traffic marked
	// by the kube-proxy to avoid double NAT bug on some kernel version
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			"meta mark", "0x4000", //TODO_TF: check the correct value when deploying kube-proxy
			"return",
		),
	})
	// don't NAT traffic within overlay network
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", podCidr,
			family, "daddr", clusterCidr,
			"return",
		),
	})
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", clusterCidr,
			family, "daddr", podCidr,
			"return",
		),
	})
	// Prevent performing Masquerade on external traffic which arrives from a Node that owns the container/pod IP address
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", "!=", podCidr,
			family, "daddr", clusterCidr,
			"return",
		),
	})
	// NAT if it's not multicast traffic
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", clusterCidr,
			family, "daddr", "!=", multicastCidr,
			masquerade,
		),
	})
	// Masquerade anything headed towards flannel from the host
	tx.Add(&knftables.Rule{
		Chain: postrtgChain,
		Rule: knftables.Concat(
			family, "saddr", "!=", clusterCidr,
			family, "daddr", clusterCidr,
			masquerade,
		),
	})
	return nil
}

// clean-up all nftables states created by flannel by deleting all related tables
func (nftm *NFTablesManager) CleanUp() error {
	log.Info("Cleaning-up simplecni tables...")
	ctx, cleanUpCancelFunc := context.WithTimeout(context.Background(), cleanUpDeadline*time.Second)
	defer cleanUpCancelFunc()
	nft, err := knftables.New(knftables.IPv4Family, ipv4Table)
	if err == nil {
		tx := nft.NewTransaction()
		tx.Delete(&knftables.Table{})
		err = nft.Run(ctx, tx)
	}
	if err != nil {
		return fmt.Errorf("nftables: couldn't delete table: %v", err)
	}

	nft, err = knftables.New(knftables.IPv6Family, ipv6Table)
	if err == nil {
		tx := nft.NewTransaction()
		tx.Delete(&knftables.Table{})
		err = nft.Run(ctx, tx)
	}
	if err != nil {
		return fmt.Errorf("nftables (ipv6): couldn't delete table: %v", err)
	}

	return nil
}
