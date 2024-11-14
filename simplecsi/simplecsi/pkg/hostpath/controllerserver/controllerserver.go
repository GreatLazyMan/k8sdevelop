package controllerserver

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"

	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/GreatLazyMan/simplecsi/pkg/hostpath"
	"github.com/GreatLazyMan/simplecsi/pkg/state"
)

type ControllerServer struct {
	csi.UnimplementedControllerServer
	*hostpath.HostPath

	// gRPC calls involving any of the fields below must be serialized
	// by locking this HostPathMutex before starting. Internal helper
	// functions assume that the HostPathMutex has been locked.
}

func NewControllerServer(hp *hostpath.HostPath) (*ControllerServer, error) {
	cs := &ControllerServer{
		HostPath: hp,
	}
	return cs, nil
}

func (hp *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	resp *csi.CreateVolumeResponse, finalErr error) {
	if err := hp.validateControllerServiceRequest(
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.V(3).Infof("invalid create volume req: %v", req)
		return nil, err
	}
	klog.Info("receive request CREATE_DELETE_VOLUME")

	if len(req.GetMutableParameters()) > 0 {
		if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_MODIFY_VOLUME); err != nil {
			klog.V(3).Infof("invalid create volume req: %v", req)
			return nil, err
		}
		klog.Infof("MutableParameters is %v, AcceptedMutableParameterNames is %v",
			req.GetMutableParameters(), hp.Config.AcceptedMutableParameterNames)
		// Check if the mutable parameters are in the accepted list
		if err := hp.validateVolumeMutableParameters(req.MutableParameters); err != nil {
			return nil, err
		}
	}

	// Check arguments
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	klog.Infof("req name is %s", req.GetName())
	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}

	// Keep a record of the requested access types.
	var accessTypeMount, accessTypeBlock bool

	for _, cap := range caps {
		if cap.GetBlock() != nil {
			accessTypeBlock = true
			klog.Info("accessTypeBlock is true")
		}
		if cap.GetMount() != nil {
			accessTypeMount = true
			klog.Info("accessTypeMount is true")
		}
	}
	// A real driver would also need to check that the other
	// fields in VolumeCapabilities are sane. The check above is
	// just enough to pass the "[Testpattern: Dynamic PV (block
	// volmode)] volumeMode should fail in binding dynamic
	// provisioned PV to PVC" storage E2E test.

	if accessTypeBlock && accessTypeMount {
		return nil, status.Error(codes.InvalidArgument, "cannot have both block and mount access type")
	}

	var requestedAccessType state.AccessType

	if accessTypeBlock {
		requestedAccessType = state.BlockAccess
	} else {
		// Default to mount.
		requestedAccessType = state.MountAccess
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	klog.Infof("capacity is %d", capacity)
	topologies := []*csi.Topology{}
	if hp.Config.EnableTopology {
		topologies = append(topologies,
			&csi.Topology{
				Segments: map[string]string{hostpath.TopologyKeyNode: hp.Config.NodeID},
			})
	}

	// Need to check for already existing volume name, and if found
	// check for the requested capacity and already allocated capacity
	if exVol, err := hp.State.GetVolumeByName(req.GetName()); err == nil {
		// Since err is nil, it means the volume with the same name already exists
		// need to check if the size of existing volume is the same as in new
		// request
		klog.Infof("%s already existing", req.GetName())
		if exVol.VolSize < capacity {
			return nil, status.Errorf(codes.AlreadyExists,
				"Volume with the same name: %s but with different size already exist", req.GetName())
		}
		if req.GetVolumeContentSource() != nil {
			volumeSource := req.VolumeContentSource
			switch volumeSource.Type.(type) {
			case *csi.VolumeContentSource_Snapshot:
				klog.Info("VolumeContentSource is Snapshot")
				if volumeSource.GetSnapshot() != nil && exVol.ParentSnapID != "" &&
					exVol.ParentSnapID != volumeSource.GetSnapshot().GetSnapshotId() {
					return nil, status.Error(codes.AlreadyExists, "existing volume source snapshot id not matching")
				}
			case *csi.VolumeContentSource_Volume:
				klog.Info("VolumeContentSource is Volume")
				if volumeSource.GetVolume() != nil && exVol.ParentVolID !=
					volumeSource.GetVolume().GetVolumeId() {
					return nil, status.Error(codes.AlreadyExists, "existing volume source volume id not matching")
				}
			default:
				return nil, status.Errorf(codes.InvalidArgument, "%v not a proper volume source", volumeSource)
			}
		}
		// TODO (sbezverk) Do I need to make sure that volume still exists?
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:           exVol.VolID,
				CapacityBytes:      int64(exVol.VolSize),
				VolumeContext:      req.GetParameters(),
				ContentSource:      req.GetVolumeContentSource(),
				AccessibleTopology: topologies,
			},
		}, nil
	}

	klog.Infof("parameters is %v", req.GetParameters())
	volumeID := uuid.NewUUID().String()
	kind := req.GetParameters()[hostpath.StorageKind]
	klog.Infof("volumeID is %s, kind is %s, name is %s", volumeID, kind, req.GetName())
	basepath := "/var/lib/"
	if bp, ok := req.GetParameters()["basepath"]; ok {
		basepath = bp
	}
	pvname := "create"
	if pvn, ok := req.GetParameters()[hostpath.PVNameKey]; ok {
		pvname = pvn
	}
	vol, err := hp.HostPathCreateVolume(volumeID, req.GetName(), basepath, pvname,
		capacity, requestedAccessType, false /* ephemeral */, kind)
	if err != nil {
		return nil, err
	}
	klog.Infof("created volume %s at path %s", vol.VolID, vol.VolPath)

	topologies = []*csi.Topology{{
		Segments: map[string]string{hostpath.TopologyKeyNode: vol.VolumeNode},
	}}
	if req.GetVolumeContentSource() != nil {
		path := hp.HostPathGetVolumePath(volumeID)
		volumeSource := req.VolumeContentSource
		switch volumeSource.Type.(type) {
		case *csi.VolumeContentSource_Snapshot:
			klog.Info("VolumeContentSource is Snapshot")
			if snapshot := volumeSource.GetSnapshot(); snapshot != nil {
				err = hp.HostPathLoadFromSnapshot(capacity, snapshot.GetSnapshotId(), path, requestedAccessType)
				vol.ParentSnapID = snapshot.GetSnapshotId()
			}
		case *csi.VolumeContentSource_Volume:
			klog.Info("VolumeContentSource is Volume")
			if srcVolume := volumeSource.GetVolume(); srcVolume != nil {
				err = hp.HostPathLoadFromVolume(capacity, srcVolume.GetVolumeId(), path, requestedAccessType)
				vol.ParentVolID = srcVolume.GetVolumeId()
			}
		default:
			err = status.Errorf(codes.InvalidArgument, "%v not a proper volume source", volumeSource)
		}
		if err != nil {
			klog.V(4).Infof("VolumeSource error: %v", err)
			if delErr := hp.HostPathDeleteVolume(volumeID); delErr != nil {
				klog.V(2).Infof("deleting hostpath volume %v failed: %v", volumeID, delErr)
			}
			return nil, err
		}
		klog.V(4).Infof("successfully populated volume %s", vol.VolID)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           volumeID,
			CapacityBytes:      req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext:      req.GetParameters(),
			ContentSource:      req.GetVolumeContentSource(),
			AccessibleTopology: topologies,
		},
	}, nil
}

func (hp *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.V(3).Infof("invalid delete volume req: %v", req)
		return nil, err
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	volId := req.GetVolumeId()
	vol, err := hp.State.GetVolumeByID(volId)
	if err != nil {
		// Volume not found: might have already deleted
		return &csi.DeleteVolumeResponse{}, nil
	}

	if vol.Attached || !vol.Published.Empty() || !vol.Staged.Empty() {
		msg := fmt.Sprintf("Volume '%s' is still used (attached: %v, staged: %v, published: %v) by '%s' node",
			vol.VolID, vol.Attached, vol.Staged, vol.Published, vol.NodeID)
		if hp.Config.CheckVolumeLifecycle {
			return nil, status.Error(codes.Internal, msg)
		}
		klog.Warning(msg)
	}

	if err := hp.HostPathDeleteVolume(volId); err != nil {
		return nil, fmt.Errorf("failed to delete volume %v: %w", volId, err)
	}
	klog.V(4).Infof("volume %v successfully deleted", volId)

	return &csi.DeleteVolumeResponse{}, nil
}

func (hp *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: hp.getControllerServiceCapabilities(),
	}, nil
}

func (hp *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}
	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, req.VolumeId)
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	if _, err := hp.State.GetVolumeByID(req.GetVolumeId()); err != nil {
		return nil, err
	}

	for _, cap := range req.GetVolumeCapabilities() {
		if cap.GetMount() == nil && cap.GetBlock() == nil {
			return nil, status.Error(codes.InvalidArgument, "cannot have both mount and block access type be undefined")
		}

		// A real driver would check the capabilities of the given volume with
		// the set of requested capabilities.
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (hp *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if !hp.Config.EnableAttach {
		return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume is not supported")
	}

	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}
	if len(req.NodeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Node ID cannot be empty")
	}
	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities cannot be empty")
	}

	if req.NodeId != hp.Config.NodeID {
		return nil, status.Errorf(codes.NotFound, "Not matching Node ID %s to hostpath Node ID %s", req.NodeId, hp.Config.NodeID)
	}

	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	vol, err := hp.State.GetVolumeByID(req.VolumeId)
	if err != nil {
		klog.Errorf("get volume by ID err: %v", err)
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// Check to see if the volume is already published.
	if vol.Attached {
		// Check if readonly flag is compatible with the publish request.
		if req.GetReadonly() != vol.ReadOnlyAttach {
			return nil, status.Error(codes.AlreadyExists, "Volume published but has incompatible readonly flag")
		}

		return &csi.ControllerPublishVolumeResponse{
			PublishContext: map[string]string{},
		}, nil
	}

	// Check attach limit before publishing.
	if hp.Config.AttachLimit > 0 && hp.GetAttachCount() >= hp.Config.AttachLimit {
		return nil, status.Errorf(codes.ResourceExhausted, "Cannot attach any more volumes to this node ('%s')", hp.Config.NodeID)
	}

	vol.Attached = true
	vol.ReadOnlyAttach = req.GetReadonly()
	if err := hp.State.UpdateVolume(vol); err != nil {
		return nil, err
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{},
	}, nil
}

func (hp *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if !hp.Config.EnableAttach {
		return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume is not supported")
	}

	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}

	// Empty node id is not a failure as per Spec
	if req.NodeId != "" && req.NodeId != hp.Config.NodeID {
		return nil, status.Errorf(codes.NotFound, "Node ID %s does not match to expected Node ID %s", req.NodeId, hp.Config.NodeID)
	}

	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	vol, err := hp.State.GetVolumeByID(req.VolumeId)
	if err != nil {
		// Not an error: a non-existent volume is not published.
		// See also https://github.com/kubernetes-csi/external-attacher/pull/165
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	// Check to see if the volume is staged/published on a node
	if !vol.Published.Empty() || !vol.Staged.Empty() {
		msg := fmt.Sprintf("Volume '%s' is still used (staged: %v, published: %v) by '%s' node",
			vol.VolID, vol.Staged, vol.Published, vol.NodeID)
		if hp.Config.CheckVolumeLifecycle {
			return nil, status.Error(codes.Internal, msg)
		}
		klog.Warning(msg)
	}

	vol.Attached = false
	if err := hp.State.UpdateVolume(vol); err != nil {
		return nil, status.Errorf(codes.Internal, "could not update volume %s: %v", vol.VolID, err)
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (hp *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// Topology and capabilities are irrelevant. We only
	// distinguish based on the "kind" parameter, if at all.
	// Without configured capacity, we just have the maximum size.
	available := hp.Config.MaxVolumeSize
	if hp.Config.Capacity.Enabled() {
		// Empty "kind" will return "zero capacity". There is no fallback
		// to some arbitrary kind here because in practice it always should
		// be set.
		kind := req.GetParameters()[hostpath.StorageKind]
		quantity := hp.Config.Capacity[kind]
		allocated := hp.SumVolumeSizes(kind)
		available = quantity.Value() - allocated
	}
	maxVolumeSize := hp.Config.MaxVolumeSize
	if maxVolumeSize > available {
		maxVolumeSize = available
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: available,
		MaximumVolumeSize: &wrapperspb.Int64Value{Value: maxVolumeSize},

		// We don't have a minimum volume size, so we might as well report that.
		// Better explicit than implicit...
		MinimumVolumeSize: &wrapperspb.Int64Value{Value: 0},
	}, nil
}

func (hp *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	volumeRes := &csi.ListVolumesResponse{
		Entries: []*csi.ListVolumesResponse_Entry{},
	}

	var (
		startIdx, volumesLength, maxLength int64
		hpVolume                           state.Volume
	)

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// Sort by volume ID.
	volumes := hp.State.GetVolumes()
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].VolID < volumes[j].VolID
	})

	if req.StartingToken == "" {
		req.StartingToken = "1"
	}

	startIdx, err := strconv.ParseInt(req.StartingToken, 10, 32)
	if err != nil {
		return nil, status.Error(codes.Aborted, "The type of startingToken should be integer")
	}

	volumesLength = int64(len(volumes))
	maxLength = int64(req.MaxEntries)

	if maxLength > volumesLength || maxLength <= 0 {
		maxLength = volumesLength
	}

	for index := startIdx - 1; index < volumesLength && index < maxLength; index++ {
		hpVolume = volumes[index]
		healthy, msg := hp.DoHealthCheckInControllerSide(hpVolume.VolID)
		klog.V(3).Infof("Healthy state: %s Volume: %t", hpVolume.VolName, healthy)
		volumeRes.Entries = append(volumeRes.Entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      hpVolume.VolID,
				CapacityBytes: hpVolume.VolSize,
			},
			Status: &csi.ListVolumesResponse_VolumeStatus{
				PublishedNodeIds: []string{hpVolume.NodeID},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: !healthy,
					Message:  msg,
				},
			},
		})
	}

	klog.V(5).Infof("Volumes are: %+v", volumeRes)
	return volumeRes, nil
}

func (hp *ControllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	volume, err := hp.State.GetVolumeByID(req.GetVolumeId())
	if err != nil {
		// ControllerGetVolume should report abnormal volume condition if volume is not found
		return &csi.ControllerGetVolumeResponse{
			Volume: &csi.Volume{
				VolumeId: req.GetVolumeId(),
			},
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  err.Error(),
				},
			},
		}, nil
	}

	healthy, msg := hp.DoHealthCheckInControllerSide(req.GetVolumeId())
	klog.V(3).Infof("Healthy state: %s Volume: %t", volume.VolName, healthy)
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volume.VolID,
			CapacityBytes: volume.VolSize,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			PublishedNodeIds: []string{volume.NodeID},
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: !healthy,
				Message:  msg,
			},
		},
	}, nil
}

func (hp *ControllerServer) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_MODIFY_VOLUME); err != nil {
		return nil, err
	}

	// Check arguments
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}
	if len(req.MutableParameters) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Mutable parameters cannot be empty")
	}

	// Check if the mutable parameters are in the accepted list
	if err := hp.validateVolumeMutableParameters(req.MutableParameters); err != nil {
		return nil, err
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// Get the volume
	_, err := hp.State.GetVolumeByID(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &csi.ControllerModifyVolumeResponse{}, nil
}

// CreateSnapshot uses tar command to create snapshot for hostpath volume. The tar command can quickly create
// archives of entire directories. The host image must have "tar" binaries in /bin, /usr/sbin, or /usr/bin.
func (hp *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT); err != nil {
		klog.V(3).Infof("invalid create snapshot req: %v", req)
		return nil, err
	}

	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	// Check arguments
	if len(req.GetSourceVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "SourceVolumeId missing in request")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// Need to check for already existing snapshot name, and if found check for the
	// requested sourceVolumeId and sourceVolumeId of snapshot that has been created.
	if exSnap, err := hp.State.GetSnapshotByName(req.GetName()); err == nil {
		// Since err is nil, it means the snapshot with the same name already exists need
		// to check if the sourceVolumeId of existing snapshot is the same as in new request.
		if exSnap.VolID == req.GetSourceVolumeId() {
			// same snapshot has been created.
			return &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SnapshotId:     exSnap.Id,
					SourceVolumeId: exSnap.VolID,
					CreationTime:   exSnap.CreationTime,
					SizeBytes:      exSnap.SizeBytes,
					ReadyToUse:     exSnap.ReadyToUse,
				},
			}, nil
		}
		return nil, status.Errorf(codes.AlreadyExists, "snapshot with the same name: %s but with different SourceVolumeId already exist", req.GetName())
	}

	volumeID := req.GetSourceVolumeId()
	ControllerServerVolume, err := hp.State.GetVolumeByID(volumeID)
	if err != nil {
		return nil, err
	}

	snapshotID := uuid.NewUUID().String()
	creationTime := timestamppb.Now()
	file := hp.GetSnapshotPath(snapshotID)
	opts, err := hostpath.OptionsFromParameters(ControllerServerVolume, req.Parameters)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume snapshot class parameters: %s", err.Error())
	}

	if err := hp.CreateSnapshotFromVolume(ControllerServerVolume, file, opts...); err != nil {
		return nil, err
	}

	klog.V(4).Infof("create volume snapshot %s", file)
	snapshot := state.Snapshot{}
	snapshot.Name = req.GetName()
	snapshot.Id = snapshotID
	snapshot.VolID = volumeID
	snapshot.Path = file
	snapshot.CreationTime = creationTime
	snapshot.SizeBytes = ControllerServerVolume.VolSize
	snapshot.ReadyToUse = true

	if err := hp.State.UpdateSnapshot(snapshot); err != nil {
		return nil, err
	}
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.Id,
			SourceVolumeId: snapshot.VolID,
			CreationTime:   snapshot.CreationTime,
			SizeBytes:      snapshot.SizeBytes,
			ReadyToUse:     snapshot.ReadyToUse,
		},
	}, nil
}

func (hp *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// Check arguments
	if len(req.GetSnapshotId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID missing in request")
	}

	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT); err != nil {
		klog.V(3).Infof("invalid delete snapshot req: %v", req)
		return nil, err
	}
	snapshotID := req.GetSnapshotId()

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// If the snapshot has a GroupSnapshotID, deletion is not allowed and should return InvalidArgument.
	if snapshot, err := hp.State.GetSnapshotByID(snapshotID); err != nil && snapshot.GroupSnapshotID != "" {
		return nil, status.Errorf(codes.InvalidArgument, "Snapshot with ID %s is part of groupsnapshot %s", snapshotID, snapshot.GroupSnapshotID)
	}

	klog.V(4).Infof("deleting snapshot %s", snapshotID)
	path := hp.GetSnapshotPath(snapshotID)
	os.RemoveAll(path)
	if err := hp.State.DeleteSnapshot(snapshotID); err != nil {
		return nil, err
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (hp *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS); err != nil {
		klog.V(3).Infof("invalid list snapshot req: %v", req)
		return nil, err
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	// case 1: SnapshotId is not empty, return snapshots that match the snapshot id,
	// none if not found.
	if len(req.GetSnapshotId()) != 0 {
		snapshotID := req.SnapshotId
		if snapshot, err := hp.State.GetSnapshotByID(snapshotID); err == nil {
			return convertSnapshot(snapshot), nil
		}
		return &csi.ListSnapshotsResponse{}, nil
	}

	// case 2: SourceVolumeId is not empty, return snapshots that match the source volume id,
	// none if not found.
	if len(req.GetSourceVolumeId()) != 0 {
		for _, snapshot := range hp.State.GetSnapshots() {
			if snapshot.VolID == req.SourceVolumeId {
				return convertSnapshot(snapshot), nil
			}
		}
		return &csi.ListSnapshotsResponse{}, nil
	}

	var snapshots []*csi.Snapshot
	// case 3: no parameter is set, so we return all the snapshots.
	hpSnapshots := hp.State.GetSnapshots()
	sort.Slice(hpSnapshots, func(i, j int) bool {
		return hpSnapshots[i].Id < hpSnapshots[j].Id
	})

	for _, snap := range hpSnapshots {
		snapshot := &csi.Snapshot{
			SnapshotId:      snap.Id,
			SourceVolumeId:  snap.VolID,
			CreationTime:    snap.CreationTime,
			SizeBytes:       snap.SizeBytes,
			ReadyToUse:      snap.ReadyToUse,
			GroupSnapshotId: snap.GroupSnapshotID,
		}
		snapshots = append(snapshots, snapshot)
	}

	var (
		ulenSnapshots = int32(len(snapshots))
		maxEntries    = req.MaxEntries
		startingToken int32
		maxToken      = uint32(math.MaxUint32)
	)

	if v := req.StartingToken; v != "" {
		i, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, status.Errorf(
				codes.Aborted,
				"startingToken=%d !< int32=%d",
				startingToken, maxToken)
		}
		startingToken = int32(i)
	}

	if startingToken > ulenSnapshots {
		return nil, status.Errorf(
			codes.Aborted,
			"startingToken=%d > len(snapshots)=%d",
			startingToken, ulenSnapshots)
	}

	// Discern the number of remaining entries.
	rem := ulenSnapshots - startingToken

	// If maxEntries is 0 or greater than the number of remaining entries then
	// set maxEntries to the number of remaining entries.
	if maxEntries == 0 || maxEntries > rem {
		maxEntries = rem
	}

	var (
		i       int
		j       = startingToken
		entries = make(
			[]*csi.ListSnapshotsResponse_Entry,
			maxEntries)
	)

	for i = 0; i < len(entries); i++ {
		entries[i] = &csi.ListSnapshotsResponse_Entry{
			Snapshot: snapshots[j],
		}
		j++
	}

	var nextToken string
	if j < ulenSnapshots {
		nextToken = fmt.Sprintf("%d", j)
	}

	return &csi.ListSnapshotsResponse{
		Entries:   entries,
		NextToken: nextToken,
	}, nil
}

func (hp *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if !hp.Config.EnableVolumeExpansion {
		return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume is not supported")
	}

	volID := req.GetVolumeId()
	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	capacity := int64(capRange.GetRequiredBytes())
	if capacity > hp.Config.MaxVolumeSize {
		return nil, status.Errorf(codes.OutOfRange, "Requested capacity %d exceeds maximum allowed %d", capacity, hp.Config.MaxVolumeSize)
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.HostPathMutex.Lock()
	defer hp.HostPathMutex.Unlock()

	exVol, err := hp.State.GetVolumeByID(volID)
	if err != nil {
		return nil, err
	}

	if exVol.VolSize < capacity {
		exVol.VolSize = capacity
		if err := hp.State.UpdateVolume(exVol); err != nil {
			return nil, err
		}
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         exVol.VolSize,
		NodeExpansionRequired: true,
	}, nil
}

func convertSnapshot(snap state.Snapshot) *csi.ListSnapshotsResponse {
	entries := []*csi.ListSnapshotsResponse_Entry{
		{
			Snapshot: &csi.Snapshot{
				SnapshotId:      snap.Id,
				SourceVolumeId:  snap.VolID,
				CreationTime:    snap.CreationTime,
				SizeBytes:       snap.SizeBytes,
				ReadyToUse:      snap.ReadyToUse,
				GroupSnapshotId: snap.GroupSnapshotID,
			},
		},
	}

	rsp := &csi.ListSnapshotsResponse{
		Entries: entries,
	}

	return rsp
}

// validateVolumeMutableParameters is a helper function to check if the mutable parameters are in the accepted list
func (hp *ControllerServer) validateVolumeMutableParameters(params map[string]string) error {
	if len(hp.Config.AcceptedMutableParameterNames) == 0 {
		return nil
	}

	accepts := sets.New(hp.Config.AcceptedMutableParameterNames...)
	unsupported := []string{}
	for k := range params {
		if !accepts.Has(k) {
			unsupported = append(unsupported, k)
		}
	}
	if len(unsupported) > 0 {
		return status.Errorf(codes.InvalidArgument, "invalid parameters: %v", unsupported)
	}
	return nil
}

func (hp *ControllerServer) validateControllerServiceRequest(
	c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range hp.getControllerServiceCapabilities() {
		if c == cap.GetRpc().GetType() {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "unsupported capability %s", c)
}

func (hp *ControllerServer) getControllerServiceCapabilities() []*csi.ControllerServiceCapability {
	var cl []csi.ControllerServiceCapability_RPC_Type
	if !hp.Config.Ephemeral {
		cl = []csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_GET_VOLUME,
			csi.ControllerServiceCapability_RPC_GET_CAPACITY,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
			csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
			csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
			csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
			csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		}
		if hp.Config.EnableVolumeExpansion && !hp.Config.DisableControllerExpansion {
			cl = append(cl, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME)
		}
		if hp.Config.EnableAttach {
			cl = append(cl, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		}
		if hp.Config.EnableControllerModifyVolume {
			cl = append(cl, csi.ControllerServiceCapability_RPC_MODIFY_VOLUME)
		}
	}

	var csc []*csi.ControllerServiceCapability

	for _, cap := range cl {
		csc = append(csc, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return csc
}
