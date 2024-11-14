package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	"github.com/GreatLazyMan/simplecsi/pkg/hostpath"
	"github.com/GreatLazyMan/simplecsi/pkg/hostpath/controllerserver"
	"github.com/GreatLazyMan/simplecsi/pkg/hostpath/identityserver"
	"github.com/GreatLazyMan/simplecsi/pkg/hostpath/nodeserver"
	"github.com/GreatLazyMan/simplecsi/pkg/k8sclient"
	"github.com/GreatLazyMan/simplecsi/pkg/state"
)

func Parse(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
		return "", "", fmt.Errorf("Invalid endpoint: %v", ep)
	}
	// Assume everything else is a file path for a Unix Domain Socket.
	return "unix", ep, nil
}

type ServerConfig struct {
	Endpoint    string
	Provisioner bool
	CsiNode     bool
}

func NewServerconfig(endpoint string) ServerConfig {
	return ServerConfig{
		Endpoint: endpoint,
	}
}

func Serve(serverConfig *ServerConfig, cfg hostpath.Config, ctx context.Context, wg *sync.WaitGroup) {
	listener, cleanup, err := Listen(serverConfig)
	if err != nil {
		klog.Errorf("Failed to listen: %v", err)
	}
	defer cleanup()
	defer klog.Infof("Quit grpc server")

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}
	server := grpc.NewServer(opts...)
	defer server.GracefulStop()

	err = os.MkdirAll(cfg.StateDir, 0750)
	if err != nil {
		wg.Done()
		return
	}
	clientSet, err := k8sclient.GetKubernetesClient()
	if err != nil {
		klog.Errorf("create k8s clientSet err: %v", err)
		wg.Done()
		return
	}
	s, err := state.New(path.Join(cfg.StateDir, "state.json"), clientSet)
	if err == nil {
		klog.Infof("Driver: %v ", cfg.DriverName)
		klog.Infof("Version: %s", cfg.VendorVersion)

		hp := hostpath.HostPath{
			Config:    &cfg,
			State:     s,
			ClientSet: clientSet,
		}
		hp.State.SetKubernetesClient(clientSet)

		identityServer, err := identityserver.NewIdentityServer(&cfg)
		if err != nil {
			klog.Errorf("create NewHostPathDriver error: %v", err)
		}
		csi.RegisterIdentityServer(server, identityServer)

		nodeServer, err := nodeserver.NewNodeServer(&hp)
		if err != nil {
			klog.Errorf("create NewHostPathDriver error: %v", err)
		}
		csi.RegisterNodeServer(server, nodeServer)

		ctlServer, err := controllerserver.NewControllerServer(&hp)
		if err != nil {
			klog.Errorf("create NewHostPathDriver error: %v", err)
		}
		csi.RegisterControllerServer(server, ctlServer)

		go server.Serve(listener)

		select {
		case <-ctx.Done():
		}
	} else {
		klog.Errorf("create new state error: %v", err)
	}
	wg.Done()
}

func Listen(opt *ServerConfig) (net.Listener, func(), error) {
	//fmt.Println(opt.Endpoint)
	proto, addr, err := Parse(opt.Endpoint)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {}
	if proto == "unix" {
		addr = "/" + addr
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) { //nolint: vetshadow
			return nil, nil, fmt.Errorf("%s: %q", addr, err)
		}
		cleanup = func() {
			klog.Info("cleanup")
			os.Remove(addr)
		}
	}

	l, err := net.Listen(proto, addr)
	return l, cleanup, err

}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	pri := klog.Level(3)
	if info.FullMethod == "/csi.v1.Identity/Probe" {
		// This call occurs frequently, therefore it only gets log at level 5.
		pri = 5
	}
	klog.V(pri).Infof("GRPC call: %s", info.FullMethod)

	v5 := klog.V(5)
	if v5.Enabled() {
		v5.Infof("GRPC request: %s", protosanitizer.StripSecrets(req))
	}
	resp, err := handler(ctx, req)
	if err != nil {
		// Always log errors. Probably not useful though without the method name?!
		klog.Errorf("GRPC error: %v", err)
	}

	if v5.Enabled() {
		v5.Infof("GRPC response: %s", protosanitizer.StripSecrets(resp))

		// In JSON format, intentionally logging without stripping secret
		// fields due to below reasons:
		// - It's technically complicated because protosanitizer.StripSecrets does
		//   not construct new objects, it just wraps the existing ones with a custom
		//   String implementation. Therefore a simple json.Marshal(protosanitizer.StripSecrets(resp))
		//   will still include secrets because it reads fields directly
		//   and more complicated code would be needed.
		// - This is indeed for verification in mock e2e tests. though
		//   currently no test which look at secrets, but we might.
		//   so conceptually it seems better to me to include secrets.
		logGRPCJson(info.FullMethod, req, resp, err)
	}

	return resp, err
}

// logGRPCJson logs the called GRPC call details in JSON format
func logGRPCJson(method string, request, reply interface{}, err error) {
	// Log JSON with the request and response for easier parsing
	logMessage := struct {
		Method   string
		Request  interface{}
		Response interface{}
		// Error as string, for backward compatibility.
		// "" on no error.
		Error string
		// Full error dump, to be able to parse out full gRPC error code and message separately in a test.
		FullError error
	}{
		Method:    method,
		Request:   request,
		Response:  reply,
		FullError: err,
	}

	if err != nil {
		logMessage.Error = err.Error()
	}

	msg, err := json.Marshal(logMessage)
	if err != nil {
		logMessage.Error = err.Error()
	}
	klog.V(5).Infof("gRPCCall: %s\n", msg)
}
