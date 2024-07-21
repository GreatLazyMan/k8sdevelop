package network

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"syscall"

	"github.com/vishvananda/netlink"
	log "k8s.io/klog/v2"
)

var (
	NoMatchIface = errors.New("nomatchiface")
	NotFoundIP   = errors.New("notfoundip")
)

func getIfaceAddrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET)
}

func getIfaceV6Addrs(iface *net.Interface) ([]netlink.Addr, error) {
	link := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET6)
}

func GetInterfaceIP4Addrs(iface *net.Interface) ([]net.IP, error) {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return nil, err
	}

	ipAddrs := make([]net.IP, 0)

	// prefer non link-local addr
	ll := make([]net.IP, 0)

	for _, addr := range addrs {
		if addr.IP.To4() == nil {
			continue
		}

		if addr.IP.IsGlobalUnicast() {
			ipAddrs = append(ipAddrs, addr.IP)
			continue
		}

		if addr.IP.IsLinkLocalUnicast() {
			ll = append(ll, addr.IP)
		}
	}

	if len(ll) > 0 {
		// didn't find global but found link-local. it'll do.
		ipAddrs = append(ipAddrs, ll...)
	}

	if len(ipAddrs) > 0 {
		return ipAddrs, nil
	}

	return nil, NotFoundIP
}

func GetInterfaceIP6Addrs(iface *net.Interface) ([]net.IP, error) {
	addrs, err := getIfaceV6Addrs(iface)
	if err != nil {
		return nil, err
	}

	ipAddrs := make([]net.IP, 0)

	// prefer non link-local addr
	ll := make([]net.IP, 0)

	for _, addr := range addrs {
		if addr.IP.To16() == nil {
			continue
		}

		if addr.IP.IsGlobalUnicast() {
			ipAddrs = append(ipAddrs, addr.IP)
			continue
		}

		if addr.IP.IsLinkLocalUnicast() {
			ll = append(ll, addr.IP)
		}
	}

	if len(ll) > 0 {
		// didn't find global but found link-local. it'll do.
		ipAddrs = append(ipAddrs, ll...)
	}

	if len(ipAddrs) > 0 {
		return ipAddrs, nil
	}

	return nil, NotFoundIP
}

func GetDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("Found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("Unable to find default route")
}

func GetInterfaceByName(ifaceName string) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Name == ifaceName {
			return &iface, nil
		}
	}

	return nil, NoMatchIface
}

func GetInterfaceByNameRegex(ifaceNameMatch string) ([]*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ifaceList := make([]*net.Interface, 0)
	ifregex, err := regexp.Compile(ifaceNameMatch)
	for _, ifaceToMatch := range ifaces {
		if ifregex.MatchString(ifaceToMatch.Name) {
			ifaceList = append(ifaceList, &ifaceToMatch)
		}
	}
	return ifaceList, nil
}

// EnsureV4AddressOnLink ensures that there is only one v4 Addr on `link` within the `ipn` address space and it equals `ipa`.
func EnsureV4AddressOnLink(ipa *net.IPNet, ipn *net.IPNet, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipa}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	var hasAddr bool
	for _, existingAddr := range existingAddrs {
		if existingAddr.Equal(addr) {
			hasAddr = true
			continue
		}

		if ipn.Contains(ipa.IP) {
			if err := netlink.AddrDel(link, &existingAddr); err != nil {
				return fmt.Errorf("failed to remove IP address %s from %s: %s", existingAddr.String(), link.Attrs().Name, err)
			}
			log.Infof("removed IP address %s from %s", ipa.IP.To4().String(), link.Attrs().Name)
		}
	}

	// Actually add the desired address to the interface if needed.
	if !hasAddr {
		if err := netlink.AddrAdd(link, &addr); err != nil {
			return fmt.Errorf("failed to add IP address %s to %s: %s", addr.String(), link.Attrs().Name, err)
		}
	}

	return nil
}

// EnsureV6AddressOnLink ensures that there is only one v6 Addr on `link` and it equals `ipn`.
// If there exist multiple addresses on link, it returns an error message to tell callers to remove additional address.
func EnsureV6AddressOnLink(ipa *net.IPNet, ipn *net.IPNet, link netlink.Link) error {
	addr := netlink.Addr{IPNet: ipa}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return err
	}

	for _, existingAddr := range existingAddrs {
		if !existingAddr.IP.IsLinkLocalUnicast() {
			if !existingAddr.Equal(addr) && ipn.Contains(ipa.IP.To16()) {
				if err := netlink.AddrDel(link, &existingAddr); err != nil {
					return fmt.Errorf("failed to remove v6 IP address %s from %s: %w", ipn.String(), link.Attrs().Name, err)
				}
			} else {
				return nil
			}
		}
	}

	// Actually add the desired address to the interface if needed.
	if err := netlink.AddrAdd(link, &addr); err != nil {
		return fmt.Errorf("failed to add v6 IP address %s to %s: %w", ipn.String(), link.Attrs().Name, err)
	}

	return nil
}
