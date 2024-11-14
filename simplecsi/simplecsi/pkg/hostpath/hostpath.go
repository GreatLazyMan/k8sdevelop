package hostpath

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	utilexec "k8s.io/utils/exec"

	"github.com/GreatLazyMan/simplecsi/pkg/k8sclient"
	"github.com/GreatLazyMan/simplecsi/pkg/state"
)

const (
	Kib    int64 = 1024
	Mib    int64 = Kib * 1024
	Gib    int64 = Mib * 1024
	Gib100 int64 = Gib * 100
	Tib    int64 = Gib * 1024
	Tib100 int64 = Tib * 100

	// storageKind is the special parameter which requests
	// storage of a certain kind (only affects capacity checks).
	StorageKind = "kind"
	// Extension with which snapshot files will be saved.
	SnapshotExt     = ".snap"
	TopologyKeyNode = "topology.hostpath.csi/node"
	DeviceID        = "deviceID"
	PVNameKey       = "csi.storage.k8s.io/pv/name"
)

var (
	VendorVersion = "dev"
)

type Config struct {
	DriverName                    string
	Endpoint                      string
	ProxyEndpoint                 string
	NodeID                        string
	VendorVersion                 string
	StateDir                      string
	MaxVolumesPerNode             int64
	MaxVolumeSize                 int64
	AttachLimit                   int64
	Capacity                      Capacity
	Ephemeral                     bool
	ShowVersion                   bool
	EnableAttach                  bool
	EnableTopology                bool
	EnableVolumeExpansion         bool
	EnableControllerModifyVolume  bool
	AcceptedMutableParameterNames StringArray
	DisableControllerExpansion    bool
	DisableNodeExpansion          bool
	MaxVolumeExpansionSizeNode    int64
	CheckVolumeLifecycle          bool
	IsAgent                       bool
	LinuxUtilsImage               string
}

type HostPath struct {
	State         state.State
	Config        *Config
	HostPathMutex sync.Mutex
	ClientSet     *kubernetes.Clientset
}

// HostPathGetVolumePath returns the canonical path for hostpath volume
func (hp *HostPath) HostPathGetVolumePath(volID string) string {
	return filepath.Join(hp.Config.StateDir, volID)
}

// getSnapshotPath returns the full path to where the snapshot is stored
func (hp *HostPath) GetSnapshotPath(snapshotID string) string {
	return filepath.Join(hp.Config.StateDir, fmt.Sprintf("%s%s", snapshotID, SnapshotExt))
}

// createVolume allocates capacity, creates the directory for the hostpath volume, and
// adds the volume to the list.
//
// It returns the volume path or err if one occurs. That error is suitable as result of a gRPC call.
func (hp *HostPath) HostPathCreateVolume(volID, name, basepath, pvname string, cap int64,
	volAccessType state.AccessType, ephemeral bool, kind string) (*state.Volume, error) {
	// Check for maximum available capacity
	if cap > hp.Config.MaxVolumeSize {
		return nil, status.Errorf(codes.OutOfRange, "Requested capacity %d exceeds maximum allowed %d", cap, hp.Config.MaxVolumeSize)
	}
	klog.Infof("hp.Config.Capacity.Enabled() %v, kind is %s", hp.Config.Capacity.Enabled(), kind)
	if hp.Config.Capacity.Enabled() {
		if kind == "" {
			// Pick some kind with sufficient remaining capacity.
			for k, c := range hp.Config.Capacity {
				if hp.SumVolumeSizes(k)+cap <= c.Value() {
					kind = k
					break
				}
			}
		}
		if kind == "" {
			// Still nothing?!
			return nil, status.Errorf(codes.ResourceExhausted, "requested capacity %d of arbitrary storage exceeds all remaining capacity", cap)
		}
		used := hp.SumVolumeSizes(kind)
		available := hp.Config.Capacity[kind]
		if used+cap > available.Value() {
			return nil, status.Errorf(codes.ResourceExhausted, "requested capacity %d exceeds remaining capacity for %q, %s out of %s already used",
				cap, kind, resource.NewQuantity(used, resource.BinarySI).String(), available.String())
		}
	} else if kind != "" {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("capacity tracking disabled, specifying kind %q is invalid", kind))
	}

	path := hp.HostPathGetVolumePath(volID)
	klog.Infof("create path %s", path)

	nodeName := ""
	var err error
	switch volAccessType {
	case state.MountAccess:
		if hp.ClientSet == nil {
			clientset, err := k8sclient.GetKubernetesClient()
			if err != nil {
				return nil, err
			}
			hp.ClientSet = clientset
		}
		//err := os.MkdirAll(path, 0777)
		nodeName, err = k8sclient.CreateJob(context.Background(), hp.ClientSet,
			pvname, "default", basepath, "", "aiops-8af5363b.ecis.huhehaote-1.cmecloud.cn/kosmos/openebs/linux-utils:3.3.0",
			[]string{"mkdir", "-p", filepath.Join(basepath, path)})
		if err != nil {
			return nil, err
		}
	case state.BlockAccess:
		executor := utilexec.New()
		size := fmt.Sprintf("%dM", cap/Mib)
		// Create a block file.
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				out, err := executor.Command("fallocate", "-l", size, path).CombinedOutput()
				if err != nil {
					return nil, fmt.Errorf("failed to create block device: %v, %v", err, string(out))
				}
			} else {
				return nil, fmt.Errorf("failed to stat block device: %v, %v", path, err)
			}
		}

		// Associate block file with the loop device.
		volPathHandler := volumepathhandler.VolumePathHandler{}
		_, err = volPathHandler.AttachFileDevice(path)
		if err != nil {
			// Remove the block file because it'll no longer be used again.
			if err2 := os.Remove(path); err2 != nil {
				klog.Errorf("failed to cleanup block file %s: %v", path, err2)
			}
			return nil, fmt.Errorf("failed to attach device %v: %v", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	volume := state.Volume{
		VolID:         volID,
		VolName:       name,
		VolSize:       cap,
		VolPath:       path,
		VolAccessType: volAccessType,
		Ephemeral:     ephemeral,
		Kind:          kind,
		VolumeNode:    nodeName,
	}
	klog.Infof("adding hostpath volume: %s = %+v", volID, volume)
	if err := hp.State.UpdateVolume(volume); err != nil {
		return nil, err
	}
	return &volume, nil
}

// HostPathDeleteVolume deletes the directory for the hostpath volume.
func (hp *HostPath) HostPathDeleteVolume(volID string) error {
	klog.V(4).Infof("starting to delete hostpath volume: %s", volID)

	vol, err := hp.State.GetVolumeByID(volID)
	if err != nil {
		// Return OK if the volume is not found.
		return nil
	}

	if vol.VolAccessType == state.BlockAccess {
		volPathHandler := volumepathhandler.VolumePathHandler{}
		path := hp.HostPathGetVolumePath(volID)
		klog.V(4).Infof("deleting loop device for file %s if it exists", path)
		if err := volPathHandler.DetachFileDevice(path); err != nil {
			return fmt.Errorf("failed to remove loop device for file %s: %v", path, err)
		}
	}

	path := hp.HostPathGetVolumePath(volID)
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := hp.State.DeleteVolume(volID); err != nil {
		return err
	}
	klog.V(4).Infof("deleted hostpath volume: %s = %+v", volID, vol)
	return nil
}

func (hp *HostPath) SumVolumeSizes(kind string) (sum int64) {
	for _, volume := range hp.State.GetVolumes() {
		if volume.Kind == kind {
			sum += volume.VolSize
		}
	}
	return
}

// HostPathIsEmpty is a simple check to determine if the specified hostpath directory
// is empty or not.
func HostPathIsEmpty(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return true, fmt.Errorf("unable to open hostpath volume, error: %v", err)
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// loadFromSnapshot populates the given destPath with data from the snapshotID
func (hp *HostPath) HostPathLoadFromSnapshot(size int64, snapshotId, destPath string, mode state.AccessType) error {
	snapshot, err := hp.State.GetSnapshotByID(snapshotId)
	if err != nil {
		return err
	}
	if !snapshot.ReadyToUse {
		return fmt.Errorf("snapshot %v is not yet ready to use", snapshotId)
	}
	if snapshot.SizeBytes > size {
		return status.Errorf(codes.InvalidArgument, "snapshot %v size %v is greater than requested volume size %v", snapshotId, snapshot.SizeBytes, size)
	}
	snapshotPath := snapshot.Path

	var cmd []string
	switch mode {
	case state.MountAccess:
		cmd = []string{"tar", "zxvf", snapshotPath, "-C", destPath}
	case state.BlockAccess:
		cmd = []string{"dd", "if=" + snapshotPath, "of=" + destPath}
	default:
		return status.Errorf(codes.InvalidArgument, "unknown accessType: %d", mode)
	}

	executor := utilexec.New()
	klog.V(4).Infof("Command Start: %v", cmd)
	out, err := executor.Command(cmd[0], cmd[1:]...).CombinedOutput()
	klog.V(4).Infof("Command Finish: %v", string(out))
	if err != nil {
		return fmt.Errorf("failed pre-populate data from snapshot %v: %w: %s", snapshotId, err, out)
	}
	return nil
}

// loadFromVolume populates the given destPath with data from the srcVolumeID
func (hp *HostPath) HostPathLoadFromVolume(size int64, srcVolumeId, destPath string, mode state.AccessType) error {
	HostPathVolume, err := hp.State.GetVolumeByID(srcVolumeId)
	if err != nil {
		return err
	}
	if HostPathVolume.VolSize > size {
		return status.Errorf(codes.InvalidArgument, "volume %v size %v is greater than requested volume size %v", srcVolumeId, HostPathVolume.VolSize, size)
	}
	if mode != HostPathVolume.VolAccessType {
		return status.Errorf(codes.InvalidArgument, "volume %v mode is not compatible with requested mode", srcVolumeId)
	}

	switch mode {
	case state.MountAccess:
		return loadFromFilesystemVolume(HostPathVolume, destPath)
	case state.BlockAccess:
		return loadFromBlockVolume(HostPathVolume, destPath)
	default:
		return status.Errorf(codes.InvalidArgument, "unknown accessType: %d", mode)
	}
}

func loadFromFilesystemVolume(HostPathVolume state.Volume, destPath string) error {
	srcPath := HostPathVolume.VolPath
	isEmpty, err := HostPathIsEmpty(srcPath)
	if err != nil {
		return fmt.Errorf("failed verification check of source hostpath volume %v: %w", HostPathVolume.VolID, err)
	}

	// If the source hostpath volume is empty it's a noop and we just move along, otherwise the cp call will fail with a a file stat error DNE
	if !isEmpty {
		args := []string{"-a", srcPath + "/.", destPath + "/"}
		executor := utilexec.New()
		out, err := executor.Command("cp", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed pre-populate data from volume %v: %s: %w", HostPathVolume.VolID, out, err)
		}
	}
	return nil
}

func loadFromBlockVolume(HostPathVolume state.Volume, destPath string) error {
	srcPath := HostPathVolume.VolPath
	args := []string{"if=" + srcPath, "of=" + destPath}
	executor := utilexec.New()
	out, err := executor.Command("dd", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed pre-populate data from volume %v: %w: %s", HostPathVolume.VolID, err, out)
	}
	return nil
}

func (hp *HostPath) GetAttachCount() int64 {
	count := int64(0)
	for _, vol := range hp.State.GetVolumes() {
		if vol.Attached {
			count++
		}
	}
	return count
}

func (hp *HostPath) CreateSnapshotFromVolume(vol state.Volume, file string, opts ...string) error {
	var args []string
	var cmdName string
	if vol.VolAccessType == state.BlockAccess {
		klog.V(4).Infof("Creating snapshot of Raw Block Mode Volume")
		cmdName = "cp"
		args = []string{vol.VolPath, file}
	} else {
		klog.V(4).Infof("Creating snapshot of Filesystem Mode Volume")
		cmdName = "tar"
		opts = append(
			[]string{"czf", file, "-C", vol.VolPath},
			opts...,
		)
		args = []string{"."}
	}
	executor := utilexec.New()
	optsAndArgs := append(opts, args...)
	out, err := executor.Command(cmdName, optsAndArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed create snapshot: %w: %s", err, out)
	}

	return nil
}

func (hp *HostPath) CreateDirByJob(name string, cmd []string) error {
	return nil
}

func (hp *HostPath) HostPathMount(pvname, basepath, nodename, srcPath, destPath string, options []string) error {

	if hp.ClientSet == nil {
		clientset, err := k8sclient.GetKubernetesClient()
		if err != nil {
			return err
		}
		hp.ClientSet = clientset
	}

	cmd := []string{"mount"}
	cmd = append(cmd, options...)
	cmd = append(cmd, filepath.Join(basepath, srcPath), destPath)

	klog.Infof("cmds is %v", cmd)
	_, err := k8sclient.CreateJob(context.Background(), hp.ClientSet,
		fmt.Sprintf("%s-mount", pvname), "default", basepath, nodename,
		"aiops-8af5363b.ecis.huhehaote-1.cmecloud.cn/kosmos/openebs/linux-utils:3.3.0",
		cmd)
	if err != nil {
		return err
	}
	return nil
}
