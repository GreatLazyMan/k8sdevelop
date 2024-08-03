// Copyright 2018 CNI authors
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

// This is a "meta-plugin". It reads in its own netconf, combines it with
// the data from flannel generated subnet file and then invokes a plugin
// like bridge or ipvlan to do the real work.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	log "github.com/sirupsen/logrus"
)

const (
	Wireguard = "wireguard"
)

// Return IPAM section for Delegate using input IPAM if present and replacing
// or complementing as needed.
func getDelegateIPAM(n *NetConf, senv *subnetEnv) (map[string]interface{}, error) {
	ipam := n.IPAM
	if ipam == nil {
		ipam = map[string]interface{}{}
	}

	if !hasKey(ipam, "type") {
		ipam["type"] = "host-local"
	}

	var rangesSlice [][]map[string]interface{}

	if senv.sn != nil && senv.sn.String() != "" {
		rangesSlice = append(rangesSlice, []map[string]interface{}{
			{"subnet": senv.sn.String()},
		},
		)
	}

	if senv.ip6Sn != nil && senv.ip6Sn.String() != "" {
		rangesSlice = append(rangesSlice, []map[string]interface{}{
			{"subnet": senv.ip6Sn.String()},
		},
		)
	}

	ipam["ranges"] = rangesSlice

	return ipam, nil
}

// generated cni Delegate/ipam object
func doCmdAdd(args *skel.CmdArgs, n *NetConf, senv *subnetEnv) error {
	ipam, err := getDelegateIPAM(n, senv)
	if !hasKey(n.Delegate, "ipMasq") {
		ipmasq := !*senv.ipmasq
		n.Delegate["ipMasq"] = ipmasq
	}
	if err != nil {
		return fmt.Errorf("failed to assemble Delegate IPAM: %w", err)
	}
	if n.CNIVersion != "" {
		n.Delegate["cniVersion"] = n.CNIVersion
	}
	n.Delegate["ipam"] = ipam

	if !hasKey(n.Delegate, "mtu") {
		mtu := senv.mtu
		n.Delegate["mtu"] = mtu
	}

	// wireguard
	if Wireguard == senv.backend {
		/*
		   "ipam": {
		       "type": "host-local",
		       "subnet": "172.18.12.0/24",
		       "rangeStart": "172.18.12.211",
		       "rangeEnd": "172.18.12.230",
		       "gateway": "172.18.12.1",
		       "routes": [
		           {
		               "dst": "0.0.0.0/0"
		           }
		       ]
		   }
		*/
		n.Delegate["name"] = "simplecni"
		n.Delegate["type"] = "ptp"
		log.WithFields(log.Fields{
			"file":     "simple_linux.go",
			"function": "doCmdAdd",
		}).Infof("type is %s", n.Delegate["type"])

		var routes []map[string]interface{}
		routes = append(routes, map[string]interface{}{
			"dst": "0.0.0.0/0",
		})
		ipam["routes"] = routes
		n.Delegate["ipam"] = ipam
	} else {
		n.Delegate["name"] = n.Name
		if !hasKey(n.Delegate, "type") {
			n.Delegate["type"] = "bridge"
		}
	}

	if n.Delegate["type"].(string) == "bridge" {
		if !hasKey(n.Delegate, "isGateway") {
			n.Delegate["isGateway"] = true
		}
	}
	fmt.Fprintf(os.Stderr, "\n%#v\n", n.Delegate)

	log.WithFields(log.Fields{
		"file":     "simple_linux.go",
		"function": "doCmdAdd",
	}).Infof("doCmdAdd args.ContainerID: %v, DataDir: %v, Delegate: %v, ipam: %v",
		args.ContainerID, n.DataDir, n.Delegate, n.Delegate["ipam"])

	return delegateAdd(args.ContainerID, n.DataDir, n.Delegate)
}

func doCmdDel(args *skel.CmdArgs, n *NetConf) error {
	cleanup, netConfBytes, err := consumeScratchNetConf(args.ContainerID, n.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Per spec should ignore error if resources are missing / already removed
			return nil
		}
		return err
	}

	// cleanup will work when no error happens
	defer func() {
		cleanup(err)
	}()

	nc := &types.NetConf{}
	if err = json.Unmarshal(netConfBytes, nc); err != nil {
		// Interface will remain in the bridge but will be removed when rebooting the node
		fmt.Fprintf(os.Stderr, "failed to parse netconf: %v", err)
		return nil
	}

	return invoke.DelegateDel(context.TODO(), nc.Type, netConfBytes, nil)
}
