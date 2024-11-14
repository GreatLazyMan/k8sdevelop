package identityserver

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/GreatLazyMan/simplecsi/pkg/hostpath"
)

type IdentityServer struct {
	csi.UnimplementedIdentityServer
	config *hostpath.Config
}

func NewIdentityServer(cfg *hostpath.Config) (*IdentityServer, error) {
	if cfg.DriverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if cfg.NodeID == "" {
		return nil, errors.New("no node id provided")
	}

	klog.Infof("Driver: %v ", cfg.DriverName)
	klog.Infof("Version: %s", cfg.VendorVersion)

	hp := &IdentityServer{
		config: cfg,
	}
	return hp, nil
}

func (hp *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.Infof("Using default GetPluginInfo")

	if hp.config.DriverName == "" {
		return nil, status.Error(codes.Unavailable, "Driver name not configured")
	}

	if hp.config.VendorVersion == "" {
		return nil, status.Error(codes.Unavailable, "Driver is missing version")
	}

	return &csi.GetPluginInfoResponse{
		Name:          hp.config.DriverName,
		VendorVersion: hp.config.VendorVersion,
	}, nil
}

func (hp *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

func (hp *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	klog.Infof("Using default capabilities")
	caps := []*csi.PluginCapability{
		{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
				},
			},
		},
		{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_GROUP_CONTROLLER_SERVICE,
				},
			},
		},
	}
	if hp.config.EnableTopology {
		caps = append(caps, &csi.PluginCapability{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
				},
			},
		})
	}

	return &csi.GetPluginCapabilitiesResponse{Capabilities: caps}, nil
}
