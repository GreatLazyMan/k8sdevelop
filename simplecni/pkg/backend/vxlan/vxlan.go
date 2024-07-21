package vxlan

import (
	"context"

	"github.com/GreatLazyMan/simplecni/pkg/nodemanager"
	"k8s.io/klog/v2"
)

const BackendType string = "vxlan"

type VxlanBackend struct {
	KubeConfig string
}

func (v *VxlanBackend) Run(ctx context.Context) {
	nodeManager, err := nodemanager.NewSubnetManager(ctx, v.KubeConfig, "simplecni.io/simplecni")
	if err != nil {
		klog.Errorf("node controller started error: %v", err)
	}
	leaseWatchChan := make(chan nodemanager.Event)
	go nodeManager.WatchLeases(ctx, leaseWatchChan)
	klog.Info("WatchLeases")
	for {
		select {
		case event := <-leaseWatchChan:
			klog.Infof("event is %v", event)
		case <-ctx.Done():
			return
		}
	}
}
