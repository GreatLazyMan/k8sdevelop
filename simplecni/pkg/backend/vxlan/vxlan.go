package vxlan

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/GreatLazyMan/simplecni/pkg/backend/iptmanager"
	"github.com/GreatLazyMan/simplecni/pkg/constants"
	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
	"github.com/GreatLazyMan/simplecni/pkg/nodemanager"
	"github.com/GreatLazyMan/simplecni/pkg/utils/files"
	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"k8s.io/klog/v2"
)

const (
	BackendType = "vxlan"
	MTU         = "1450"
	MTUint      = 1450
	DevieName   = "simplevxlan"
	VxlanIP     = "VxlanIP"
	VxlanMac    = "VxlanMac"
)

type VxlanBackend struct {
	KubeConfig string
	Config     *netconfig.Config
	Device     *network.VxlanDevice
}

func (v *VxlanBackend) HandleEvent(event nodemanager.Event) {

	// This route is used when traffic should be vxlan encapsulated

	if len(event.Lease.CidrIPv4) == 0 {
		klog.Warning("CidrIPv4 is empty")
		return
	}
	var vxlanAddr net.IP
	var vxlanMac net.HardwareAddr
	var deviceAddr net.IP

	vxlanAddrKey := strings.ToLower(fmt.Sprintf("%s%s", constants.Prefix, VxlanIP))
	if vxlanAddrStr, ok := event.Lease.AttrMap[vxlanAddrKey]; ok {
		vxlanAddr = net.ParseIP(vxlanAddrStr)
	} else {
		klog.Warningf("can't find key %s", vxlanAddrKey)
		return
	}

	vxlanMacKey := strings.ToLower(fmt.Sprintf("%s%s", constants.Prefix, VxlanMac))
	if vxlanMacStr, ok := event.Lease.AttrMap[vxlanMacKey]; ok {
		vxlanMac, _ = net.ParseMAC(vxlanMacStr)
	} else {
		klog.Errorf("can't find key %s", vxlanMacKey)
		return
	}

	deviceAddrKey := strings.ToLower(fmt.Sprintf("%s%s", constants.Prefix, netconfig.IPAddr))
	if deviceAddrStr, ok := event.Lease.AttrMap[deviceAddrKey]; ok {
		deviceAddr = net.ParseIP(deviceAddrStr)
	} else {
		klog.Errorf("can't find key %s", deviceAddrKey)
		return
	}

	switch event.Type {
	case nodemanager.EventAdded:
		klog.Infof("config perr vxlan: %v, %v, local aadr: %v, cidr: %v", vxlanAddr, vxlanMac, deviceAddr, event.Lease.CidrIPv4)
		v.Device.ConfigurePeer(&vxlanAddr, vxlanMac, &deviceAddr, event.Lease.CidrIPv4[0])

	case nodemanager.EventRemoved:
		klog.Infof("removd perr vxlan: %v, %v, local aadr: %v, cidr: %v", vxlanAddr, vxlanMac, deviceAddr, event.Lease.CidrIPv4)
		v.Device.RemovePeer(&vxlanAddr, vxlanMac, &deviceAddr, event.Lease.CidrIPv4[0])
	}
}

func (v *VxlanBackend) GetSubnetMap(lease *nodemanager.Lease) map[string]string {
	subnetMap := make(map[string]string)
	subnetMap["MTU"] = MTU
	if len(lease.CidrIPv4) > 0 {
		subnetMap[constants.SUBNET] = lease.CidrIPv4[0].String()
	}
	if len(lease.CidrIPv6) > 0 {
		subnetMap[constants.IPV6_SUBNET] = lease.CidrIPv6[0].String()
	}
	subnetMap["IPMASQ"] = "true"
	return subnetMap
}

func (v *VxlanBackend) Run(ctx context.Context) {
	var err error
	klog.Info("vxlan backend run")
	nodeManager, err := nodemanager.NewSubnetManager(ctx, v.KubeConfig)
	if err != nil {
		klog.Errorf("node controller started error: %v", err)
		return
	}

	// get node info
	leaseWatchChan := make(chan nodemanager.Event)
	lease, err := nodeManager.AcquireLease(ctx)
	if err != nil {
		klog.Errorf("acquire node info error: %v", err)
		return
	}

	// iptables or nfstables
	klog.Infof("env is %s", os.Getenv("IPTABLES"))
	if os.Getenv("IPTABLES") != constants.NFTABLES {
		var iptManager iptmanager.IPTablesManager
		err = iptManager.Init(ctx, lease.ClusterCidrIPv4, lease.CidrIPv4[0], lease.ClusterCidrIPv6, nil)
	} else {
		var nftManager iptmanager.NFTablesManager
		err = nftManager.Init(ctx, lease.ClusterCidrIPv4, lease.CidrIPv4[0], lease.ClusterCidrIPv6, nil)
	}
	if err != nil {
		klog.Errorf("init iptables manager err: %v", err)
		return
	}

	// write subnet file info
	subnetMap := v.GetSubnetMap(lease)
	err = files.WriteSubnetFile(subnetMap)
	if err != nil {
		klog.Errorf("write subnetfile error: %v", err)
		return
	}

	// init vxlan devie
	vAttr := network.VxlanDeviceAttrs{
		Name:      DevieName,
		Vni:       10086,
		VtepIndex: v.Config.Netlink.Attrs().Index,
		VtepAddr:  net.ParseIP(v.Config.Configmap[netconfig.IPAddr]).To4(),
		MTU:       v.Config.Netlink.Attrs().MTU,
	}

	vxlanDevice, err := network.NewVXLANDevice(&vAttr)
	if err != nil {
		klog.Errorf("init vxlan device error: %v", err)
		return
	}
	v.Device = vxlanDevice

	vxlanAddr := lease.CidrIPv4[0]
	vxlanAddr.Mask = network.Localhost.Mask
	err = vxlanDevice.Configure(vxlanAddr, lease.CidrIPv4[0])
	if err != nil {
		klog.Errorf("configure vxlan device error: %v", err)
		return
	}

	v.Config.Configmap[VxlanMac] = vxlanDevice.Link.HardwareAddr.String()
	v.Config.Configmap[VxlanIP] = lease.CidrIPv4[0].IP.To4().String()
	err = nodeManager.CompleteLease(ctx, v.Config.Configmap)
	if err != nil {
		klog.Errorf("completelease, patch node annotations or status error: %v", err)
		return
	}

	// start watch node
	go nodeManager.WatchLeases(ctx, leaseWatchChan)
	klog.Info("Start Watching Leases")

	// handle node object key
	for {
		select {
		case event := <-leaseWatchChan:
			klog.Infof("event is %v", event)
			v.HandleEvent(event)
		case <-ctx.Done():
			close(leaseWatchChan)
			return
		}
	}
}
