package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/GreatLazyMan/simplecni/pkg/utils/util"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
)

const (
	ExitSetupSuccess = 0
	ExitSetupFailed  = 1

	ENV_WG_TUN_FD             = "WG_TUN_FD"
	ENV_WG_UAPI_FD            = "WG_UAPI_FD"
	ENV_WG_PROCESS_FOREGROUND = "WG_PROCESS_FOREGROUND"

	DefaultDeviceName = "wg0"
	DefaultPrivateKey = "eBdjVFnLDJKn5iPE5taZBFplzdP+4hqjmjGp/x5SJ1w="
	DefaultPublicKey  = "7XaAsFfFjjDYQqwnFxNxlQzgv6OVCN7g7y+eZlO/vFs="
)

var (
	DefaultWgPort int = 51820
	TimeInterval      = time.Second
)

type WireguarDevice struct {
	Link      *netlink.Link
	Client    *wgctrl.Client
	Name      string
	PublicKey string
}

var DefaultWgPublicKey wgtypes.Key
var DefaultWgPrivateKey wgtypes.Key

func init() {
	DefaultWgPrivateKey, _ = wgtypes.ParseKey(DefaultPublicKey)

}

func NewWireguardDevice(ctx context.Context, cancel context.CancelFunc, ipn *net.IPNet) (*WireguarDevice, error) {
	interfaceName := DefaultDeviceName

	// open TUN device (or use supplied fd)
	tdev, err := func() (tun.Device, error) {
		tunFdStr := os.Getenv(ENV_WG_TUN_FD)
		if tunFdStr == "" {
			return tun.CreateTUN(interfaceName, device.DefaultMTU)
		}
		// construct tun device from supplied fd
		fd, err := strconv.ParseUint(tunFdStr, 10, 32)
		if err != nil {
			return nil, err
		}
		err = unix.SetNonblock(int(fd), true)
		if err != nil {
			return nil, err
		}
		file := os.NewFile(uintptr(fd), "")
		return tun.CreateTUNFromFile(file, device.DefaultMTU)
	}()

	if err == nil {
		realInterfaceName, err2 := tdev.Name()
		if err2 == nil {
			interfaceName = realInterfaceName
		}
	}

	if err != nil {
		klog.Errorf("Failed to create TUN device: %v", err)
		return nil, fmt.Errorf("setup failed: %v", err)
	}
	klog.Info("created tun device")

	// open UAPI file (or use supplied fd)
	fileUAPI, err := func() (*os.File, error) {
		uapiFdStr := os.Getenv(ENV_WG_UAPI_FD)
		if uapiFdStr == "" {
			return ipc.UAPIOpen(interfaceName)
		}
		// use supplied fd
		fd, err := strconv.ParseUint(uapiFdStr, 10, 32)
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(fd), ""), nil
	}()

	if err != nil {
		klog.Errorf("UAPI listen error: %v", err)
		return nil, fmt.Errorf("setup failed: %v", err)
	}
	klog.Info("created uapi")
	// daemonize the process

	logger := device.NewLogger(
		device.LogLevelError,
		fmt.Sprintf("(%s) ", interfaceName),
	)

	bind := conn.NewDefaultBind()
	device := device.NewDevice(tdev, bind, logger)

	uapi, err := ipc.UAPIListen(interfaceName, fileUAPI)
	if err != nil {
		logger.Errorf("Failed to listen on uapi socket: %v", err)
		os.Exit(ExitSetupFailed)
	}
	go backgroudRun(cancel, uapi, device)

	klog.Info("Wireguard Device started")
	wireguardLink, err := ensureWgLink(ctx, interfaceName, ipn)
	if err != nil {
		return nil, fmt.Errorf("ensure wireguard link err: %v", err)
	}
	client, wgdevice, err := configWgServer(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("config wireguard server err: %v", err)
	}

	return &WireguarDevice{
		Link:      wireguardLink,
		Client:    client,
		Name:      interfaceName,
		PublicKey: wgdevice.PublicKey.String(),
	}, nil
}

func ensureWgLink(ctx context.Context, interfaceName string, ipn *net.IPNet) (*netlink.Link, error) {
	var wireguardLink *netlink.Link
	var err error
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
			wireguardLink, err = getdevice(interfaceName)
			if err != nil {
				return nil, fmt.Errorf("get wireguard device err: %v", err)
			}
		}
		if wireguardLink != nil {
			break
		}
		time.Sleep(10 * time.Microsecond)
		klog.Info("try to get wireguard link")
	}
	addr := netlink.Addr{IPNet: ipn}
	if err := netlink.AddrAdd(*wireguardLink, &addr); err != nil {
		return nil, fmt.Errorf("failed to add IP address %s to %s: %s", addr.String(), (*wireguardLink).Attrs().Name, err)
	}
	if err := netlink.LinkSetUp(*wireguardLink); err != nil {
		return nil, fmt.Errorf("failed to set interface %s to UP state: %s", (*wireguardLink).Attrs().Name, err)
	}
	klog.Infof("wireguard device is inited: %v", (*wireguardLink).Attrs())

	return wireguardLink, nil
}

func configWgServer(interfaceName string) (*wgctrl.Client, *wgtypes.Device, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, nil, fmt.Errorf("create wireguard client err: %v", err)
	}

	klog.Info("start config privateKey")
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("generate privatekey error: %v", err)
	}
	config := wgtypes.Config{
		PrivateKey: &privateKey,
	}
	err = client.ConfigureDevice(interfaceName, config)
	if err != nil {
		klog.Errorf("config wireguard device error: %v", err)
		return nil, nil, fmt.Errorf("config wireguard device error: %v", err)
	}
	klog.Info("finish config privateKey")

	wgdevice, err := client.Device(interfaceName)
	if err != nil {
		return nil, nil, fmt.Errorf("get wireguard device error: %v", err)
	}

	klog.Info("start config bind port")
	config = wgtypes.Config{
		ListenPort: &DefaultWgPort,
	}
	if err := client.ConfigureDevice(interfaceName, config); err != nil {
		klog.Fatalf("can't config device port: %v", err)
	}
	klog.Info("finish config bind port")

	return client, wgdevice, nil
}

func getdevice(deviceName string) (*netlink.Link, error) {

	existing, err := netlink.LinkByName(deviceName)
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func peerToPeerConfig(peer wgtypes.Peer) wgtypes.PeerConfig {
	return wgtypes.PeerConfig{
		PublicKey:                   peer.PublicKey,
		PresharedKey:                &peer.PresharedKey,
		Endpoint:                    peer.Endpoint,
		PersistentKeepaliveInterval: &peer.PersistentKeepaliveInterval,
		ReplaceAllowedIPs:           false, // 根据需要设置
		AllowedIPs:                  peer.AllowedIPs,
	}
}

func peerToPeerConfigSlice(peers []wgtypes.Peer) []wgtypes.PeerConfig {
	wgConfig := make([]wgtypes.PeerConfig, 0)
	for _, peer := range peers {
		wgConfig = append(wgConfig, peerToPeerConfig(peer))
	}
	return wgConfig
}

func peerToPeerConfigSliceExcludePeer(peers []wgtypes.Peer, peerIP *net.IP) []wgtypes.PeerConfig {
	wgConfig := make([]wgtypes.PeerConfig, 0)
	for _, peer := range peers {
		if peer.Endpoint.IP.Equal(*peerIP) {
			continue
		}
		wgConfig = append(wgConfig, peerToPeerConfig(peer))
	}
	return wgConfig
}

func (w *WireguarDevice) AddPeers(peerIP *net.IP, peerCidr *net.IPNet, publicKey string) error {
	device, err := w.Client.Device(DefaultDeviceName)
	if err != nil {
		return fmt.Errorf("get wireguard device error: %v", err)
	}
	parsedPublicKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("parse public key: %v", err)
	}

	mask := net.CIDRMask(32, 32)
	peerIPCidr := net.IPNet{
		IP:   *peerIP,
		Mask: mask,
	}

	//https://github.com/WireGuard/wgctrl-go/issues/146
	// PublicKey can't be same server's PublicKey
	peerConfigs := peerToPeerConfigSlice(device.Peers)
	peerConfig := wgtypes.PeerConfig{
		PublicKey: parsedPublicKey,
		Endpoint:  &net.UDPAddr{IP: *peerIP, Port: DefaultWgPort},
		AllowedIPs: []net.IPNet{
			*peerCidr,
			peerIPCidr,
		},
		PersistentKeepaliveInterval: &TimeInterval,
	}
	peerConfigs = append(peerConfigs, peerConfig)

	klog.Infof("add a new peer: %v", peerConfigs)

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}
	err = w.Client.ConfigureDevice(w.Name, config)
	if err != nil {
		return fmt.Errorf("config peers error: %v", err)
	}

	device, err = w.Client.Device(DefaultDeviceName)
	if err != nil {
		return fmt.Errorf("get wireguard device error: %v", err)
	}
	klog.Infof("added peer, now device is: %v", device)
	route := netlink.Route{
		LinkIndex: (*w.Link).Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       peerCidr,
	}

	//route.SetFlag(syscall.RTNH_F_ONLINK)

	if err := util.Retry(func() error {
		return netlink.RouteReplace(&route)
	}); err != nil {
		return err
	}

	return nil
}

func (w *WireguarDevice) DelPeers(peerIP *net.IP) error {
	device, err := w.Client.Device(DefaultDeviceName)
	if err != nil {
		return fmt.Errorf("get wireguard device error: %v", err)
	}

	peerConfigs := peerToPeerConfigSliceExcludePeer(device.Peers, peerIP)
	config := wgtypes.Config{
		Peers: peerConfigs,
	}

	err = w.Client.ConfigureDevice(w.Name, config)
	if err != nil {
		return fmt.Errorf("config peers error: %v", err)
	}
	return nil
}

func backgroudRun(cancel context.CancelFunc, uapi net.Listener, device *device.Device) {

	errs := make(chan error)
	term := make(chan os.Signal, 1)
	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				errs <- err
				cancel()
				return
			}
			go device.IpcHandle(conn)
		}
	}()

	klog.Info("UAPI listener started")

	// wait for program to terminate
	signal.Notify(term, unix.SIGTERM)
	signal.Notify(term, os.Interrupt)

	select {
	case <-term:
	case <-errs:
	case <-device.Wait():
	}

	// clean up
	uapi.Close()
	device.Close()
	cancel()

	klog.Info("Shutting down")
}
