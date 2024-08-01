package backend

import (
	"context"
	"os"

	"github.com/GreatLazyMan/simplecni/cmd/options"
	"github.com/GreatLazyMan/simplecni/pkg/backend/vxlan"
	"github.com/GreatLazyMan/simplecni/pkg/backend/wireguard"
	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
	"k8s.io/klog/v2"
)

type NetworkBackend interface {
	Run(context.Context)
}

func NewNetworkBackend(opts *options.CmdLineOpts) NetworkBackend {
	var backendType string
	netConf, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		klog.Errorf("failed to read net conf: %v", err)
		return nil
	}
	sc, err := netconfig.ParseConfig(string(netConf))
	if err != nil {
		klog.Errorf("error parsing subnet config: %s", err)
		return nil
	}
	backendType = sc.BackendType

	switch backendType {
	case vxlan.BackendType:
		return &vxlan.VxlanBackend{KubeConfig: opts.KubeConfig, Config: sc}
	case wireguard.BackendType:
		return &wireguard.WireguardBackend{KubeConfig: opts.KubeConfig, Config: sc, Ctx: opts.Ctx, Cancel: opts.Cancel}
	default:
		return &vxlan.VxlanBackend{KubeConfig: opts.KubeConfig, Config: sc}
	}
}
