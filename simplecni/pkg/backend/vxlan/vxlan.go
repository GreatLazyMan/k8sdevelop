package vxlan

import (
	"context"
)

const BackendType string = "vxlan"

type VxlanBackend struct {
}

func (*VxlanBackend) Run(ctx context.Context) error {
	return nil
}
