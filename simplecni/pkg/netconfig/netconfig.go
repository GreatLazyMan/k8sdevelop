package netconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/GreatLazyMan/simplecni/pkg/utils/files"
	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
)

type Config struct {
	BackendType string            `json:"-"`
	Configmap   map[string]string `json:"-"`
	Backend     json.RawMessage   `json:",omitempty"`
	IfaceName   string            `json:"-"`
	Netlink     netlink.Link      `json:"-"`
}

const (
	IPAddr   = "IPAddr"
	IPAddrV6 = "IPAddrV6"
)

func setInterfaceAddressByName(ifaceName string, ifaceNameList []string, ifaceNameRegex string, cfg *Config) error {
	configmap := make(map[string]string)
	// acquire addr by iface name
	ifaceList := make([]*net.Interface, 0)
	if len(ifaceName) == 0 && len(ifaceNameList) == 0 && len(ifaceNameRegex) == 0 {
		iface, err := network.GetDefaultGatewayInterface()
		if err != nil {
			return fmt.Errorf("get get default gateway interface error: %v", err)
		}
		ifaceList = append(ifaceList, iface)

	} else {
		allIfaceNameList := make([]string, 0)
		if len(ifaceName) != 0 {
			allIfaceNameList = append(allIfaceNameList, ifaceName)
		}
		if len(ifaceNameList) != 0 {
			allIfaceNameList = append(allIfaceNameList, ifaceNameList...)
		}
		for _, ifaceName := range allIfaceNameList {
			iface, err := network.GetInterfaceByName(ifaceName)
			if err != nil {
				if errors.Is(err, network.NoMatchIface) {
					return fmt.Errorf("get iface %s error by name: %v", ifaceName, err)
				} else {
					klog.Warningf("can't get iface by name %s", ifaceName)
					continue
				}
			} else {
				ifaceList = append(ifaceList, iface)
			}
		}
		if len(ifaceList) != 0 {
			ifaceListRegex, err := network.GetInterfaceByNameRegex(ifaceNameRegex)
			if err != nil {
				return fmt.Errorf("get regex iface %s error : %v", ifaceNameRegex, err)
			}
			ifaceList = append(ifaceList, ifaceListRegex...)
		}
	}
	for _, iface := range ifaceList {
		// get ipv4 addr
		ipaddr, err := network.GetInterfaceIP4Addrs(iface)
		if err != nil {
			klog.Errorf("get ipv4 addr from  %s error: %v", iface.Name, err)
			continue
		}
		configmap[IPAddr] = ipaddr[0].String()
		//set link
		link, err := netlink.LinkByName(iface.Name)
		if err != nil {
			return fmt.Errorf("get netlink by name %s error: %v", iface.Name, err)
		}
		cfg.Netlink = link

		// get ipv6 addr
		ipaddr, err = network.GetInterfaceIP6Addrs(iface)
		if err != nil && !errors.Is(err, network.NotFoundIP) {
			return fmt.Errorf("get ipv6 addr error: %v", err)
		}
		if len(ipaddr) > 0 {
			configmap[IPAddrV6] = ipaddr[0].String()
		}
	}
	cfg.Configmap = configmap
	if _, ok := configmap[IPAddr]; !ok {
		return fmt.Errorf("get address from iface error from %v", ifaceList)
	}
	return nil
}

func parseBackendType(cfg *Config) error {
	var bt struct {
		Type       string
		Iface      string
		IfaceList  []string
		IfaceMatch string
	}
	be := cfg.Backend
	if err := json.Unmarshal(be, &bt); err != nil {
		return fmt.Errorf("error decoding Backend property of config: %v", err)
	}
	cfg.BackendType = bt.Type

	klog.Infof("get json: %v", bt)
	err := setInterfaceAddressByName(bt.Iface, bt.IfaceList, bt.IfaceMatch, cfg)
	if err != nil {
		return fmt.Errorf("get ip address error: %v", err)
	}

	return nil
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	err = parseBackendType(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func ReadCIDRFromSubnetFile(CIDRKey string) (*net.IPNet, error) {
	prevCIDRString, err := files.ReadKeyFromSubnetFile(CIDRKey)
	if err != nil {
		return nil, fmt.Errorf("read %s from subnetfile error: %v", CIDRKey, err)
	}
	cidrs := strings.Split(prevCIDRString, ",")

	if len(cidrs) == 0 {
		klog.Warningf("no subnet found for key %s ", CIDRKey)
		return nil, nil
	} else if len(cidrs) > 1 {
		klog.Errorf("error reading subnet: more than 1 entry found for key %s: %v", CIDRKey, cidrs)
		return nil, err
	} else {

		_, prevCIDRs, _ := net.ParseCIDR(cidrs[0])
		return prevCIDRs, nil
	}
}
