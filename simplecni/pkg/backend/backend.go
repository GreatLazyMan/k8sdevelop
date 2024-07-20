package backend

import (
	"context"

	"github.com/GreatLazyMan/simplecni/pkg/backend/vxlan"
	"github.com/GreatLazyMan/simplecni/pkg/netconfig"
)

type NetworkBackend interface {
	Run(context.Context) error
}

func NewNetworkBackend(sc *netconfig.Config) NetworkBackend {
	switch sc.BackendType {
	case vxlan.BackendType:
		return &vxlan.VxlanBackend{}
	default:
		return &vxlan.VxlanBackend{}
	}
}
