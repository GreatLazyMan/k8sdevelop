package wireguard

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/GreatLazyMan/simplecni/pkg/constants"
	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
	"github.com/GreatLazyMan/simplecni/pkg/nodemanager"
	"github.com/GreatLazyMan/simplecni/pkg/utils/files"
	"github.com/GreatLazyMan/simplecni/pkg/utils/network"
	"k8s.io/klog/v2"
)

const (
	BackendType = "wireguard"
	WgPublicKey = "WgPublicKey"
)

type WireguardBackend struct {
	KubeConfig string
	Config     *netconfig.Config
	Ctx        context.Context
	Device     *network.WireguarDevice
	Cancel     context.CancelFunc
}

func (w *WireguardBackend) HandleEvent(event nodemanager.Event) {

	// This route is used when traffic should be vxlan encapsulated

	if len(event.Lease.CidrIPv4) == 0 {
		klog.Warning("CidrIPv4 is empty")
		return
	}
	var deviceAddr net.IP
	deviceAddrKey := strings.ToLower(fmt.Sprintf("%s%s", constants.Prefix, netconfig.IPAddr))
	if deviceAddrStr, ok := event.Lease.AttrMap[deviceAddrKey]; ok {
		deviceAddr = net.ParseIP(deviceAddrStr)
	} else {
		klog.Errorf("can't find key %s", deviceAddrKey)
		return
	}
	wgPublicKey := strings.ToLower(fmt.Sprintf("%s%s", constants.Prefix, WgPublicKey))

	switch event.Type {
	case nodemanager.EventAdded:
		klog.Infof("add peer: %v %v %v", deviceAddr, event.Lease.CidrIPv4, event.Lease.AttrMap)
		w.Device.AddPeers(&deviceAddr, event.Lease.CidrIPv4[0], event.Lease.AttrMap[wgPublicKey])

	case nodemanager.EventRemoved:
		klog.Infof("del peer: %v %v", deviceAddr, event.Lease.CidrIPv4)
		w.Device.DelPeers(&deviceAddr)
	}
}

func (w *WireguardBackend) GetSubnetMap(lease *nodemanager.Lease) map[string]string {
	subnetMap := make(map[string]string)
	if len(lease.CidrIPv4) > 0 {
		subnetMap[constants.SUBNET] = lease.CidrIPv4[0].String()
	}
	if len(lease.CidrIPv6) > 0 {
		subnetMap[constants.IPV6_SUBNET] = lease.CidrIPv6[0].String()
	}
	subnetMap["IPMASQ"] = "true"
	subnetMap["BACKEND"] = "wireguard"
	return subnetMap
}

func (w *WireguardBackend) Run(ctx context.Context) {

	var err error
	klog.Info("wireguard backend run")
	nodeManager, err := nodemanager.NewSubnetManager(ctx, w.KubeConfig)
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

	//// iptables or nfstables
	//klog.Infof("env is %s", os.Getenv("IPTABLES"))
	//if os.Getenv("IPTABLES") != constants.NFTABLES {
	//	var iptManager iptmanager.IPTablesManager
	//	err = iptManager.Init(ctx, lease.ClusterCidrIPv4, lease.CidrIPv4[0], lease.ClusterCidrIPv6, nil)
	//} else {
	//	var nftManager iptmanager.NFTablesManager
	//	err = nftManager.Init(ctx, lease.ClusterCidrIPv4, lease.CidrIPv4[0], lease.ClusterCidrIPv6, nil)
	//}
	//if err != nil {
	//	klog.Errorf("init iptables manager err: %v", err)
	//	return
	//}

	// write subnet file info
	subnetMap := w.GetSubnetMap(lease)
	err = files.WriteSubnetFile(subnetMap)
	if err != nil {
		klog.Errorf("write subnetfile error: %v", err)
		return
	}
	klog.Info("write subnetMap")

	// init wireguard devie
	wgdevice, err := network.NewWireguardDevice(w.Ctx, w.Cancel, lease.CidrIPv4[0])
	if err != nil {
		klog.Errorf("init wireguard device error: %v", err)
		return
	}
	w.Device = wgdevice
	klog.Info("backend wireguard device is inited")

	w.Config.Configmap[WgPublicKey] = wgdevice.PublicKey
	err = nodeManager.CompleteLease(ctx, w.Config.Configmap)
	if err != nil {
		klog.Errorf("completelease, patch node annotations or status error: %v", err)
		return
	}
	// start watch node
	klog.Info("Start Watching Leases")
	go nodeManager.WatchLeases(ctx, leaseWatchChan)

	// handle node object key
	for {
		select {
		case event := <-leaseWatchChan:
			klog.Infof("event is %v", event)
			w.HandleEvent(event)
		case <-ctx.Done():
			close(leaseWatchChan)
			return
		}
	}
}
