// Copyright 2015 CNI authors
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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	log "github.com/sirupsen/logrus"
)

const (
	defaultSubnetFile = "/var/run/simplecni/subnet.env"
	defaultDataDir    = "/var/lib/cni/simple"
)

var (
	Program   string
	Version   string
	Commit    string
	buildDate string
)

type NetConf struct {
	types.NetConf

	// IPAM field "replaces" that of types.NetConf which is incomplete
	IPAM          map[string]interface{} `json:"ipam,omitempty"`
	SubnetFile    string                 `json:"subnetFile"`
	DataDir       string                 `json:"dataDir"`
	Delegate      map[string]interface{} `json:"delegate"`
	RuntimeConfig map[string]interface{} `json:"runtimeConfig,omitempty"`
}

type subnetEnv struct {
	nws     []*net.IPNet
	sn      *net.IPNet
	ip6Nws  []*net.IPNet
	ip6Sn   *net.IPNet
	mtu     *uint
	ipmasq  *bool
	backend string
}

func init() {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat:           "2006-01-02 15:04:05",
		ForceColors:               true,
		EnvironmentOverrideColors: true,
		FullTimestamp:             true,
		DisableLevelTruncation:    true,
	})
	logfile, _ := os.OpenFile("/var/log/simplecniplugin.log", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	log.SetOutput(logfile)
}

func (se *subnetEnv) missing() string {
	m := []string{}

	//if len(se.nws) == 0 && len(se.ip6Nws) == 0 {
	//	m = append(m, []string{"NETWORK", "IPV6_NETWORK"}...)
	//}
	if se.sn == nil && se.ip6Sn == nil {
		m = append(m, []string{"SUBNET", "IPV6_SUBNET"}...)
	}
	//if se.mtu == nil {
	//	m = append(m, "MTU")
	//}
	if se.ipmasq == nil {
		m = append(m, "IPMASQ")
	}
	return strings.Join(m, ", ")
}

func loadNetConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{
		SubnetFile: defaultSubnetFile,
		DataDir:    defaultDataDir,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}
	return n, nil
}

func isSubnetAlreadyPresent(nws []*net.IPNet, nw *net.IPNet) bool {
	compareMask := func(m1 net.IPMask, m2 net.IPMask) bool {
		for i := range m1 {
			if m1[i] != m2[i] {
				return false
			}
		}
		return true
	}
	for _, nwi := range nws {
		if nw.IP.Equal(nwi.IP) && compareMask(nw.Mask, nwi.Mask) {
			return true
		}
	}
	return false
}

func loadSubnetEnv(fn string) (*subnetEnv, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	se := &subnetEnv{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), "=", 2)
		switch parts[0] {
		case "NETWORK":
			cidrs := strings.Split(parts[1], ",")
			se.nws = make([]*net.IPNet, 0, len(cidrs))
			for i := range cidrs {
				_, nw, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					return nil, err
				}
				if !isSubnetAlreadyPresent(se.nws, nw) {
					se.nws = append(se.nws, nw)
				}
			}

		case "SUBNET":
			_, se.sn, err = net.ParseCIDR(parts[1])
			if err != nil {
				return nil, err
			}

		case "IPV6_NETWORK":
			cidrs := strings.Split(parts[1], ",")
			se.ip6Nws = make([]*net.IPNet, 0, len(cidrs))
			for i := range cidrs {
				_, ip6nw, err := net.ParseCIDR(cidrs[i])
				if err != nil {
					return nil, err
				}
				if !isSubnetAlreadyPresent(se.ip6Nws, ip6nw) {
					se.ip6Nws = append(se.ip6Nws, ip6nw)
				}
			}

		case "IPV6_SUBNET":
			_, se.ip6Sn, err = net.ParseCIDR(parts[1])
			if err != nil {
				return nil, err
			}

		case "MTU":
			mtu, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				return nil, err
			}
			se.mtu = new(uint)
			*se.mtu = uint(mtu)

		case "IPMASQ":
			ipmasq := parts[1] == "true"
			se.ipmasq = &ipmasq

		case "BACKEND":
			//log.WithFields(log.Fields{
			//	"file":     "simple.go",
			//	"function": "loadSubnetEnv",
			//}).Info("2222222222222222222")
			se.backend = parts[1]
		}
	}
	if err := s.Err(); err != nil {
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "loadSubnetEnv",
		}).Errorf("bufio error: %v", err)
		return nil, err
	}

	if m := se.missing(); m != "" {
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "loadSubnetEnv",
		}).Errorf("%v is missing %v", fn, m)
		return nil, fmt.Errorf("%v is missing %v", fn, m)
	}

	log.WithFields(log.Fields{
		"file":     "simple.go",
		"function": "loadSubnetEnv",
	}).Infof("subnetEnv is: %v ", se)

	return se, nil
}

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dataDir, containerID)
	return writeAndSyncFile(path, netconf, 0600)
}

// WriteAndSyncFile behaves just like ioutil.WriteFile in the standard library,
// but calls Sync before closing the file. WriteAndSyncFile guarantees the data
// is synced if there is no error returned.
func writeAndSyncFile(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

func consumeScratchNetConf(containerID, dataDir string) (func(error), []byte, error) {
	path := filepath.Join(dataDir, containerID)

	// cleanup will do clean job when no error happens in consuming/using process
	cleanup := func(err error) {
		if err == nil {
			// Ignore errors when removing - Per spec safe to continue during DEL
			_ = os.Remove(path)
		}
	}
	netConfBytes, err := os.ReadFile(path)

	return cleanup, netConfBytes, err
}

// let delegated cni plugin do its work
func delegateAdd(cid, dataDir string, netconf map[string]interface{}) error {
	netconfBytes, err := json.Marshal(netconf)
	fmt.Fprintf(os.Stderr, "delegateAdd: netconf sent to delegate plugin:\n")
	os.Stderr.Write(netconfBytes)
	if err != nil {
		return fmt.Errorf("error serializing delegate netconf: %v", err)
	}

	// save the rendered netconf for cmdDel
	if err = saveScratchNetConf(cid, dataDir, netconfBytes); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"file":     "simple.go",
		"function": "delegateAdd",
	}).Infof("netconf is: %v ", netconf)
	result, err := invoke.DelegateAdd(context.TODO(), netconf["type"].(string), netconfBytes, nil)
	if err != nil {
		err = fmt.Errorf("failed to delegate add: %w", err)
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "delegateAdd",
		}).Errorf("invoke.DelegateAdd error: %v", err)
		return err
	}
	ret := result.Print()
	if ret != nil {
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "delegateAdd",
		}).Infof("result is: %v ", ret)
	}
	return ret
}

func hasKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

func isString(i interface{}) bool {
	_, ok := i.(string)
	return ok
}

// load config and input
func cmdAdd(args *skel.CmdArgs) error {
	n, err := loadNetConf(args.StdinData)
	log.WithFields(log.Fields{
		"file":     "simple.go",
		"function": "cmdAdd",
	}).Infof("input is: %v ", n)

	if err != nil {
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "cmdAdd",
		}).Errorf("loadNetConf failed error: %v ", err)
		return fmt.Errorf("loadNetConf failed: %w", err)
	}
	fenv, err := loadSubnetEnv(n.SubnetFile)
	if err != nil {
		log.WithFields(log.Fields{
			"file":     "simple.go",
			"function": "cmdAdd",
		}).Errorf("loadSubnetEnv failed error: %v ", err)
		return fmt.Errorf("loadSubnetEnv failed: %w", err)
	}

	if n.Delegate == nil {
		n.Delegate = make(map[string]interface{})
	} else {
		if hasKey(n.Delegate, "type") && !isString(n.Delegate["type"]) {
			return fmt.Errorf("'delegate' dictionary, if present, must have (string) 'type' field")
		}
		if hasKey(n.Delegate, "name") {
			return fmt.Errorf("'delegate' dictionary must not have 'name' field, it'll be set by flannel")
		}
		if hasKey(n.Delegate, "ipam") {
			return fmt.Errorf("'delegate' dictionary must not have 'ipam' field, it'll be set by flannel")
		}
	}

	return doCmdAdd(args, n, fenv)
}

func cmdDel(args *skel.CmdArgs) error {
	nc, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	return doCmdDel(args, nc)
}

type pluginInfo struct {
	CNIVersion_        string   `json:"cniVersion"`
	SupportedVersions_ []string `json:"supportedVersions,omitempty"`
}

func (p *pluginInfo) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(p)
}

func (p *pluginInfo) SupportedVersions() []string {
	return p.SupportedVersions_
}

func main() {
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:   cmdAdd,
		Del:   cmdDel,
		Check: cmdCheck,
	},
		&pluginInfo{
			SupportedVersions_: []string{"0.3.1"},
		},
		"CNI plugin simple v0.0.1",
	)
}

func cmdCheck(args *skel.CmdArgs) error {
	// TODO: implement
	return nil
}
