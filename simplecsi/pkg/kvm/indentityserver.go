// Node Plugin 和 the Controller Plugin 都需要此服务
package kvm

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

type IdentityServerEmbed struct {
	IdentityServer
	csi.UnsafeIdentityServer
}

type IdentityServer struct {
}

func NewIdentityServer() *IdentityServerEmbed {
	return &IdentityServerEmbed{}
}

// GetPluginInfo 返回插件信息
func (ids *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.V(4).Infof("GetPluginInfo: called with args %+v", req.String())

	return &csi.GetPluginInfoResponse{
		Name:          driverName,
		VendorVersion: version,
	}, nil
}

// GetPluginCapabilities 返回插件支持的功能
func (ids *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	klog.V(4).Infof("GetPluginCapabilities: called with args %+v", req.String())
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
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
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}

	return resp, nil
}

// Probe 插件健康检测
func (ids *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	klog.V(4).Infof("Probe: called with args %+v", req.String())
	return &csi.ProbeResponse{}, nil
}
