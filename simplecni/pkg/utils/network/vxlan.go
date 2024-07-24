//go:build !windows
// +build !windows

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

package network

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/util/validation/field"
	log "k8s.io/klog/v2"

	"github.com/GreatLazyMan/simplecni/pkg/utils/util"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
)

type VxlanDeviceAttrs struct {
	Vni       uint32
	Name      string
	MTU       int
	VtepIndex int
	VtepAddr  net.IP
	VtepPort  int
	HwAddr    net.HardwareAddr
}

type VxlanDevice struct {
	Link *netlink.Vxlan
}

func NewVXLANDevice(devAttrs *VxlanDeviceAttrs) (*VxlanDevice, error) {
	var err error
	hardwareAddr := devAttrs.HwAddr
	if devAttrs.HwAddr == nil {
		hardwareAddr, err = newHardwareAddr()
		if err != nil {
			return nil, err
		}
	}

	link := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:         devAttrs.Name,
			HardwareAddr: hardwareAddr,
			MTU:          devAttrs.MTU - 50,
		},
		VxlanId:      int(devAttrs.Vni),
		VtepDevIndex: devAttrs.VtepIndex,
		SrcAddr:      devAttrs.VtepAddr,
		Port:         devAttrs.VtepPort,
		Learning:     false,
	}
	log.Info("starting to create vxlan device")

	link, err = ensureLink(link)
	if err != nil {
		return nil, err
	}

	_, _ = sysctl.Sysctl(fmt.Sprintf("net/ipv6/conf/%s/accept_ra", devAttrs.Name), "0")

	return &VxlanDevice{
		Link: link,
	}, nil
}

func ensureLink(vxlan *netlink.Vxlan) (*netlink.Vxlan, error) {
	err := netlink.LinkAdd(vxlan)
	if err == syscall.EEXIST {
		// it's ok if the device already exists as long as config is similar
		log.V(1).Infof("VXLAN device already exists")
		existing, err := netlink.LinkByName(vxlan.Name)
		if err != nil {
			return nil, err
		}

		incompat := vxlanLinksIncompat(vxlan, existing)
		if incompat == "" {
			log.V(1).Infof("Returning existing device")
			return existing.(*netlink.Vxlan), nil
		}

		// delete existing
		log.Warningf("%q already exists with incompatible configuration: %v; recreating device", vxlan.Name, incompat)
		if err = netlink.LinkDel(existing); err != nil {
			return nil, fmt.Errorf("failed to delete interface: %v", err)
		}

		// create new
		if err = netlink.LinkAdd(vxlan); err != nil {
			return nil, fmt.Errorf("failed to create vxlan interface: %v", err)
		}
	} else if err != nil {
		return nil, err
	}

	ifindex := vxlan.Index
	link, err := netlink.LinkByIndex(vxlan.Index)
	if err != nil {
		return nil, fmt.Errorf("can't locate created vxlan device with index %v", ifindex)
	}

	var ok bool
	if vxlan, ok = link.(*netlink.Vxlan); !ok {
		return nil, fmt.Errorf("created vxlan device with index %v is not vxlan", ifindex)
	}
	log.Info("created vxlan device")

	return vxlan, nil
}

func (dev *VxlanDevice) Configure(ipa, ipn *net.IPNet) error {
	if err := EnsureV4AddressOnLink(ipa, ipn, dev.Link); err != nil {
		return fmt.Errorf("failed to ensure address of interface %s: %s", dev.Link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(dev.Link); err != nil {
		return fmt.Errorf("failed to set interface %s to UP state: %s", dev.Link.Attrs().Name, err)
	}

	// ensure vxlan device hadware mac
	// See https://github.com/flannel-io/flannel/issues/1795
	nLink, err := netlink.LinkByName(dev.Link.LinkAttrs.Name)
	if err == nil {
		if vxlan, ok := nLink.(*netlink.Vxlan); ok {
			if vxlan.Attrs().HardwareAddr.String() != dev.MACAddr().String() {
				return fmt.Errorf("%s's mac address wanted: %s, but got: %v", dev.Link.Name, dev.MACAddr().String(), vxlan.HardwareAddr)
			}
		}
	}

	return nil
}

func (dev *VxlanDevice) ConfigureIPv6(ipa, ipn *net.IPNet) error {
	if err := EnsureV6AddressOnLink(ipa, ipn, dev.Link); err != nil {
		return fmt.Errorf("failed to ensure v6 address of interface %s: %w", dev.Link.Attrs().Name, err)
	}

	if err := netlink.LinkSetUp(dev.Link); err != nil {
		return fmt.Errorf("failed to set v6 interface %s to UP state: %w", dev.Link.Attrs().Name, err)
	}

	// ensure vxlan device hadware mac
	// See https://github.com/flannel-io/flannel/issues/1795
	nLink, err := netlink.LinkByName(dev.Link.LinkAttrs.Name)
	if err == nil {
		if vxlan, ok := nLink.(*netlink.Vxlan); ok {
			if vxlan.Attrs().HardwareAddr.String() != dev.MACAddr().String() {
				return fmt.Errorf("%s's v6 mac address wanted: %s, but got: %v", dev.Link.Name, dev.MACAddr().String(), vxlan.HardwareAddr)
			}
		}
	}

	return nil
}

func (dev *VxlanDevice) MACAddr() net.HardwareAddr {
	return dev.Link.HardwareAddr
}

type neighbor struct {
	MAC net.HardwareAddr
	IP  *net.IP
	IP6 *net.IP
}

func (dev *VxlanDevice) AddFDB(n neighbor) error {
	log.V(4).Infof("calling AddFDB: %v, %v", n.IP, n.MAC)
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           *n.IP,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) AddV6FDB(n neighbor) error {
	log.V(4).Infof("calling AddV6FDB: %v, %v", n.IP6, n.MAC)
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           *n.IP6,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) DelFDB(n neighbor) error {
	log.V(4).Infof("calling DelFDB: %v, %v", n.IP, n.MAC)
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           *n.IP,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) DelV6FDB(n neighbor) error {
	log.V(4).Infof("calling DelV6FDB: %v, %v", n.IP6, n.MAC)
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           *n.IP6,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) AddARP(n neighbor) error {
	log.V(4).Infof("calling AddARP: %v, %v", n.IP, n.MAC)
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           *n.IP,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) AddV6ARP(n neighbor) error {
	log.V(4).Infof("calling AddV6ARP: %v, %v", n.IP6, n.MAC)
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           *n.IP6,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) DelARP(n neighbor) error {
	log.V(4).Infof("calling DelARP: %v, %v", n.IP, n.MAC)
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           *n.IP,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) DelV6ARP(n neighbor) error {
	log.V(4).Infof("calling DelV6ARP: %v, %v", n.IP6, n.MAC)
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    dev.Link.Index,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           *n.IP6,
		HardwareAddr: n.MAC,
	})
}

func (dev *VxlanDevice) ConfigurePeer(vxlanAddr *net.IP, vxlanMac net.HardwareAddr, deviceAddr *net.IP, dst *net.IPNet) error {
	if err := util.Retry(func() error {
		return dev.AddARP(neighbor{IP: vxlanAddr, MAC: vxlanMac})
	}); err != nil {
		log.Error("AddARP failed: ", err)
		return err
	}

	if err := util.Retry(func() error {
		return dev.AddFDB(neighbor{IP: deviceAddr, MAC: vxlanMac})
	}); err != nil {
		log.Error("AddFDB failed: ", err)
		// Try to clean up the ARP entry then continue
		if err := util.Retry(func() error {
			return dev.DelARP(neighbor{IP: vxlanAddr, MAC: vxlanMac})
		}); err != nil {
			log.Error("DelARP failed: ", err)
		}
		return err
	}

	// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
	// this is done last.
	vxlanRoute := netlink.Route{
		LinkIndex: dev.Link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       dst,
		Gw:        *vxlanAddr,
	}
	vxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)

	if err := util.Retry(func() error {
		return netlink.RouteReplace(&vxlanRoute)
	}); err != nil {

		// Try to clean up both the ARP and FDB entries then continue
		if err := dev.DelARP(neighbor{IP: vxlanAddr, MAC: vxlanMac}); err != nil {
			log.Error("DelARP failed: ", err)
		}
		if err := dev.DelFDB(neighbor{IP: deviceAddr, MAC: vxlanMac}); err != nil {
			log.Error("DelFDB failed: ", err)
		}

		return err
	}
	return nil
}

func (dev *VxlanDevice) RemovePeer(vxlanAddr *net.IP, vxlanMac net.HardwareAddr, deviceAddr *net.IP, dst *net.IPNet) error {
	errs := field.ErrorList{}
	if err := util.Retry(func() error {
		return dev.DelARP(neighbor{IP: vxlanAddr, MAC: vxlanMac})
	}); err != nil {
		log.Error("DelARP failed: ", err)
		errs = append(errs, &field.Error{Field: err.Error()})
	}

	if err := util.Retry(func() error {
		return dev.DelFDB(neighbor{IP: deviceAddr, MAC: vxlanMac})
	}); err != nil {
		log.Error("DelFDB failed: ", err)
		errs = append(errs, &field.Error{Field: err.Error()})
	}

	vxlanRoute := netlink.Route{
		LinkIndex: dev.Link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       dst,
		Gw:        *vxlanAddr,
	}
	vxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)
	if err := util.Retry(func() error {
		return netlink.RouteDel(&vxlanRoute)
	}); err != nil {
		log.Errorf("failed to delete vxlanRoute (%s -> %s): %v", vxlanRoute.Dst, vxlanRoute.Gw, err)
		errs = append(errs, &field.Error{Field: err.Error()})
	}
	return errs.ToAggregate()
}

func vxlanLinksIncompat(l1, l2 netlink.Link) string {
	if l1.Type() != l2.Type() {
		return fmt.Sprintf("link type: %v vs %v", l1.Type(), l2.Type())
	}

	v1 := l1.(*netlink.Vxlan)
	v2 := l2.(*netlink.Vxlan)

	if v1.VxlanId != v2.VxlanId {
		return fmt.Sprintf("vni: %v vs %v", v1.VxlanId, v2.VxlanId)
	}

	if v1.VtepDevIndex > 0 && v2.VtepDevIndex > 0 && v1.VtepDevIndex != v2.VtepDevIndex {
		return fmt.Sprintf("vtep (external) interface: %v vs %v", v1.VtepDevIndex, v2.VtepDevIndex)
	}

	if len(v1.SrcAddr) > 0 && len(v2.SrcAddr) > 0 && !v1.SrcAddr.Equal(v2.SrcAddr) {
		return fmt.Sprintf("vtep (external) IP: %v vs %v", v1.SrcAddr, v2.SrcAddr)
	}

	if len(v1.Group) > 0 && len(v2.Group) > 0 && !v1.Group.Equal(v2.Group) {
		return fmt.Sprintf("group address: %v vs %v", v1.Group, v2.Group)
	}

	if v1.L2miss != v2.L2miss {
		return fmt.Sprintf("l2miss: %v vs %v", v1.L2miss, v2.L2miss)
	}

	if v1.Port > 0 && v2.Port > 0 && v1.Port != v2.Port {
		return fmt.Sprintf("port: %v vs %v", v1.Port, v2.Port)
	}

	if v1.GBP != v2.GBP {
		return fmt.Sprintf("gbp: %v vs %v", v1.GBP, v2.GBP)
	}

	return ""
}
