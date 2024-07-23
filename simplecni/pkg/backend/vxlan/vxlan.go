package vxlan

import (
	"context"
	"net"

	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
	"github.com/GreatLazyMan/simplecni/pkg/nodemanager"
	"github.com/GreatLazyMan/simplecni/pkg/utils/files"
	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"github.com/cloudflare/cfssl/log"
	"k8s.io/klog/v2"
)

const (
	BackendType = "vxlan"
	MTU         = "1450"
	MTUint      = 1450
	DevieName   = "simplevxlan"
)

type VxlanBackend struct {
	KubeConfig string
	Config     *netconfig.Config
}

func (v *VxlanBackend) ConfigureRoute(event nodemanager.Event) {

	switch event.Type {
	case nodemanager.EventAdded:
		log.Infof("Subnet added: %v", event.Lease.CidrIPv4[0])
		//routeAdd(route, netlink.FAMILY_V4, n.addToRouteList, n.removeFromV4RouteList)
	case nodemanager.EventRemoved:
		log.Infof("Subnet del: %v", event.Lease.CidrIPv4[0])
		//n.removeFromV4RouteList(*route)
	default:
		log.Error("Internal error: unknown event type: ", int(event.Type))
	}

}

func (v *VxlanBackend) GetSubnetMap(lease *nodemanager.Lease) map[string]string {
	subnetMap := make(map[string]string)
	subnetMap["MTU"] = MTU
	if len(lease.CidrIPv4) > 0 {
		subnetMap["SUBNET"] = lease.CidrIPv4[0].String()
	}
	if len(lease.CidrIPv6) > 0 {
		subnetMap["IPV6_SUBNET"] = lease.CidrIPv6[0].String()
	}
	return subnetMap
}
func (v *VxlanBackend) Run(ctx context.Context) {
	klog.Info("vxlan backend run")
	nodeManager, err := nodemanager.NewSubnetManager(ctx, v.KubeConfig)
	if err != nil {
		klog.Errorf("node controller started error: %v", err)
		return
	}

	// get node info
	leaseWatchChan := make(chan nodemanager.Event)
	lease, err := nodeManager.AcquireLease(ctx, v.Config.Configmap)
	if err != nil {
		klog.Errorf("acquire node info error: %v", err)
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

	vlanxDevice, err := network.NewVXLANDevice(&vAttr)
	if err != nil {
		klog.Errorf("init vxlan device error: %v", err)
		return
	}

	vxlanAddr := lease.CidrIPv4[0]
	vxlanAddr.Mask = network.Localhost.Mask
	err = vlanxDevice.Configure(vxlanAddr, lease.CidrIPv4[0])
	if err != nil {
		klog.Errorf("configure vxlan device error: %v", err)
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
			v.ConfigureRoute(event)
		case <-ctx.Done():
			close(leaseWatchChan)
			return
		}
	}
}
