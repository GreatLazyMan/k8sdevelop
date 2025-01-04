package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	DeviceNumber        = 2
	ResourceName string = "simple.io/fuse"
	KubeletSock  string = "kubelet.sock"
	FuseSock            = "fuse.sock"
)

type Server struct {
	Watcher       *fsnotify.Watcher
	Devices       []*pluginapi.Device
	FuseSockPath  string
	KubeletSock   string
	StopChannel   chan any
	HealthChannel chan *pluginapi.Device
	GrpcServer    *grpc.Server
}

func NewServer(file string) Server {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Fatalf("new watcher error: %v", err)
		panic(err)
	}
	if err := watcher.Add(file); err != nil {
		klog.Fatalf("add watcher error: %v", err)
		panic(err)
	}
	PluginSockPath := file + FuseSock
	KubeletSockPath := file + KubeletSock

	devs := []*pluginapi.Device{}

	hostname, _ := os.Hostname()
	for i := 0; i < DeviceNumber; i++ {
		devs = append(devs, &pluginapi.Device{
			ID:     fmt.Sprintf("fuse-%s-%d", hostname, i),
			Health: pluginapi.Healthy,
		})
	}
	grpcSercer := grpc.NewServer([]grpc.ServerOption{}...)
	return Server{
		Watcher:       watcher,
		FuseSockPath:  PluginSockPath,
		KubeletSock:   KubeletSockPath,
		Devices:       devs,
		GrpcServer:    grpcSercer,
		StopChannel:   make(chan any),
		HealthChannel: make(chan *pluginapi.Device),
	}
}

func (s *Server) Cleanup() error {
	if err := syscall.Unlink(s.FuseSockPath); err != nil && !os.IsNotExist(err) {
		klog.Errorf("remove %s error: %v", s.FuseSockPath, err)
		return err
	}
	return nil
}

// TODO: 不可用，后续学习grpc
func (s *Server) Dial(ctx context.Context, sock string, timeout time.Duration) (*grpc.ClientConn, error) {
	client, err := grpc.NewClient(sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if deadline, ok := ctx.Deadline(); ok {
				return net.DialTimeout("unix", addr, time.Until(deadline))
			}
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	for {
		gstate := client.GetState()
		klog.Infof("gstate is %v", gstate)
		if gstate == connectivity.Idle {
			klog.Info("start connect")
			client.Connect()
			klog.Info("finish connect")
		}
		if gstate == connectivity.Ready {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		klog.Errorf("create grpc client err: %v", err)
		return nil, err
	}
	return client, nil
}

func (s *Server) deviceExists(id string) bool {
	for _, d := range s.Devices {
		if d.ID == id {
			return true
		}
	}
	return false
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (s *Server) Register(ctx context.Context) error {
	conn, err := connect(s.KubeletSock, 5*time.Second)
	// conn, err := connect(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		klog.Error("Dial error: %v", err)
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(s.FuseSockPath),
		ResourceName: ResourceName,
	}

	_, err = client.Register(ctx, reqt)
	if err != nil {
		klog.Errorf("Register error: %v", err)
		return err
	}
	return nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.Cleanup(); err != nil {
		klog.Errorf("clearup error %v", err)
		return err
	}
	pluginapi.RegisterDevicePluginServer(s.GrpcServer, s)
	sock, err := net.Listen("unix", s.FuseSockPath)
	if err != nil {
		klog.Errorf("listen sock error: %v", err)
		return err
	}
	go s.GrpcServer.Serve(sock)
	conn, err := connect(s.FuseSockPath, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	klog.Info("listen sock succeed")

	return nil
}

// dial establishes the gRPC communication with the registered device plugin.
func connect(socketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := grpc.DialContext(ctx, socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if deadline, ok := ctx.Deadline(); ok {
				return net.DialTimeout("unix", addr, time.Until(deadline))
			}
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s *Server) Serve(ctx context.Context) {
	if err := s.Start(ctx); err != nil {
		klog.Fatalf("start grpc server error: %v", err)
		panic(err)
	}
	if err := s.Register(ctx); err != nil {
		klog.Fatalf("registers resourceName %s error: %v", ResourceName, err)
		panic(err)
	}
	klog.Info("Registered device plugin with Kubelet")
	<-ctx.Done()
}

func (s *Server) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (s *Server) ListAndWatch(empty *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	server.Send(&pluginapi.ListAndWatchResponse{Devices: s.Devices})

	for {
		// TODO: 可以增加健康检查
		select {
		case <-s.StopChannel:
			return nil
		case d := <-s.HealthChannel:
			d.Health = pluginapi.Unhealthy
			server.Send(&pluginapi.ListAndWatchResponse{Devices: s.Devices})
		}
	}

}

func (s *Server) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var responses pluginapi.AllocateResponse

	for _, req := range reqs.ContainerRequests {
		for _, id := range req.DevicesIDs {
			if !s.deviceExists(id) {
				return nil, fmt.Errorf("invalid allocation request: unknown device: %s", id)
			}
			klog.Infof("allocate id %v", id)

			response := pluginapi.ContainerAllocateResponse{
				Envs: map[string]string{
					"fuser": "fuser",
				},
			}
			response.Devices = []*pluginapi.DeviceSpec{
				{
					ContainerPath: "/dev/fuse",
					HostPath:      "/dev/fuse",
					Permissions:   "rwm",
				},
			}

			responses.ContainerResponses = append(responses.ContainerResponses, &response)
		}
	}
	return &responses, nil
}

func (s *Server) PreStartContainer(ctx context.Context, request *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (s *Server) GetPreferredAllocation(ctx context.Context, request *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}
