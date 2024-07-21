package netconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"k8s.io/klog/v2"
)

type Config struct {
	BackendType string            `json:"-"`
	Configmap   map[string]string `json:"-"`
	Backend     json.RawMessage   `json:",omitempty"`
}

const (
	IPAddr   = "IPAddr"
	IPAddrV6 = "IPAddrV6"
)

func setInterfaceAddressByName(ifaceName string, ifaceNameList []string, ifaceNameRegex string, configmap map[string]string) error {
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
		ipaddr, err := network.GetInterfaceIP4Addrs(iface)
		if err != nil {
			klog.Errorf("get ipv4 addr from  %s error: %v", iface.Name, err)
			continue
		}
		configmap[IPAddr] = ipaddr[0].String()
		ipaddr, err = network.GetInterfaceIP6Addrs(iface)
		if err != nil && !errors.Is(err, network.NotFoundIP) {
			return fmt.Errorf("get ipv6 addr error: %v", err)
		}
		if len(ipaddr) > 0 {
			configmap[IPAddrV6] = ipaddr[0].String()
		}
	}
	if _, ok := configmap[IPAddr]; !ok {
		return fmt.Errorf("get address from iface error from %v", ifaceList)
	}
	return nil
}

func parseBackendType(be json.RawMessage) (string, map[string]string, error) {
	var bt struct {
		Type       string
		Iface      string
		IfaceList  []string
		IfaceMatch string
	}
	cm := make(map[string]string)
	if err := json.Unmarshal(be, &bt); err != nil {
		return "", nil, fmt.Errorf("error decoding Backend property of config: %v", err)
	}

	klog.Infof("get json: %v", bt)
	err := setInterfaceAddressByName(bt.Iface, bt.IfaceList, bt.IfaceMatch, cm)
	if err != nil {
		return "", nil, fmt.Errorf("get ip address error: %v", err)
	}

	klog.Infof("get backend type %s, configmap %v", bt.Type, cm)
	return bt.Type, cm, nil
}

func ParseConfig(s string) (*Config, error) {
	cfg := new(Config)
	err := json.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	bt, cm, err := parseBackendType(cfg.Backend)
	if err != nil {
		return nil, err
	}
	cfg.BackendType = bt
	cfg.Configmap = cm

	return cfg, nil
}
