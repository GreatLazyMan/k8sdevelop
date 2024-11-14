package groupcontrollerserver

import (
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"

	"github.com/GreatLazyMan/simplecsi/pkg/hostpath"
	"github.com/GreatLazyMan/simplecsi/pkg/state"
)

type GroupControllerServer struct {
	csi.UnimplementedGroupControllerServer
	hostpath.HostPath
	// gRPC calls involving any of the fields below must be serialized
	// by locking this mutex before starting. Internal helper
	// functions assume that the mutex has been locked.
	mutex sync.Mutex
}

func (hp *GroupControllerServer) GroupControllerGetCapabilities(context.Context, *csi.GroupControllerGetCapabilitiesRequest) (*csi.GroupControllerGetCapabilitiesResponse, error) {
	return &csi.GroupControllerGetCapabilitiesResponse{
		Capabilities: []*csi.GroupControllerServiceCapability{{
			Type: &csi.GroupControllerServiceCapability_Rpc{
				Rpc: &csi.GroupControllerServiceCapability_RPC{
					Type: csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT,
				},
			},
		}},
	}, nil
}

func (hp *GroupControllerServer) CreateVolumeGroupSnapshot(ctx context.Context, req *csi.CreateVolumeGroupSnapshotRequest) (*csi.CreateVolumeGroupSnapshotResponse, error) {
	if err := hp.validateGroupControllerServiceRequest(csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT); err != nil {
		klog.V(3).Infof("invalid create volume group snapshot req: %v", req)
		return nil, err
	}

	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Name missing in request")
	}
	// Check arguments
	if len(req.GetSourceVolumeIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "SourceVolumeIds missing in request")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	// Need to check for already existing groupsnapshot name, and if found check for the
	// requested sourceVolumeIds and sourceVolumeIds of groupsnapshot that has been created.
	if exGS, err := hp.State.GetGroupSnapshotByName(req.GetName()); err == nil {
		// Since err is nil, it means the groupsnapshot with the same name already exists. Need
		// to check if the sourceVolumeIds of existing groupsnapshot is the same as in new request.

		if !exGS.MatchesSourceVolumeIDs(req.GetSourceVolumeIds()) {
			return nil, status.Errorf(codes.AlreadyExists, "group snapshot with the same name: %s but with different SourceVolumeIds already exist", req.GetName())
		}

		// same groupsnapshot has been created.
		snapshots := make([]*csi.Snapshot, len(exGS.SnapshotIDs))
		readyToUse := true

		for i, snapshotID := range exGS.SnapshotIDs {
			snapshot, err := hp.State.GetSnapshotByID(snapshotID)
			if err != nil {
				return nil, err
			}

			snapshots[i] = &csi.Snapshot{
				SizeBytes:       snapshot.SizeBytes,
				CreationTime:    snapshot.CreationTime,
				ReadyToUse:      snapshot.ReadyToUse,
				GroupSnapshotId: snapshot.GroupSnapshotID,
			}

			readyToUse = readyToUse && snapshot.ReadyToUse
		}

		return &csi.CreateVolumeGroupSnapshotResponse{
			GroupSnapshot: &csi.VolumeGroupSnapshot{
				GroupSnapshotId: exGS.Id,
				Snapshots:       snapshots,
				CreationTime:    exGS.CreationTime,
				ReadyToUse:      readyToUse,
			},
		}, nil
	}

	groupSnapshot := state.GroupSnapshot{
		Name:            req.GetName(),
		Id:              uuid.NewUUID().String(),
		CreationTime:    timestamppb.Now(),
		SnapshotIDs:     make([]string, len(req.GetSourceVolumeIds())),
		SourceVolumeIDs: make([]string, len(req.GetSourceVolumeIds())),
		ReadyToUse:      true,
	}

	copy(groupSnapshot.SourceVolumeIDs, req.GetSourceVolumeIds())

	snapshots := make([]*csi.Snapshot, len(req.GetSourceVolumeIds()))

	// TODO: defer a cleanup function to remove snapshots in case of a failure

	for i, volumeID := range req.GetSourceVolumeIds() {
		GroupControllerServerVolume, err := hp.State.GetVolumeByID(volumeID)
		if err != nil {
			return nil, err
		}

		opts, err := hostpath.OptionsFromParameters(GroupControllerServerVolume, req.Parameters)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid volume group snapshot class parameters: %s", err.Error())
		}

		snapshotID := uuid.NewUUID().String()
		file := hp.GetSnapshotPath(snapshotID)

		if err := hp.CreateSnapshotFromVolume(GroupControllerServerVolume, file, opts...); err != nil {
			return nil, err
		}

		klog.V(4).Infof("create volume snapshot %s", file)
		snapshot := state.Snapshot{}
		snapshot.Name = req.GetName() + "-" + volumeID
		snapshot.Id = snapshotID
		snapshot.VolID = volumeID
		snapshot.Path = file
		snapshot.CreationTime = groupSnapshot.CreationTime
		snapshot.SizeBytes = GroupControllerServerVolume.VolSize
		snapshot.ReadyToUse = true
		snapshot.GroupSnapshotID = groupSnapshot.Id

		hp.State.UpdateSnapshot(snapshot)

		groupSnapshot.SnapshotIDs[i] = snapshotID

		snapshots[i] = &csi.Snapshot{
			SizeBytes:       GroupControllerServerVolume.VolSize,
			SnapshotId:      snapshotID,
			SourceVolumeId:  volumeID,
			CreationTime:    groupSnapshot.CreationTime,
			ReadyToUse:      true,
			GroupSnapshotId: groupSnapshot.Id,
		}
	}

	if err := hp.State.UpdateGroupSnapshot(groupSnapshot); err != nil {
		return nil, err
	}

	return &csi.CreateVolumeGroupSnapshotResponse{
		GroupSnapshot: &csi.VolumeGroupSnapshot{
			GroupSnapshotId: groupSnapshot.Id,
			Snapshots:       snapshots,
			CreationTime:    groupSnapshot.CreationTime,
			ReadyToUse:      groupSnapshot.ReadyToUse,
		},
	}, nil
}

func (hp *GroupControllerServer) DeleteVolumeGroupSnapshot(ctx context.Context, req *csi.DeleteVolumeGroupSnapshotRequest) (*csi.DeleteVolumeGroupSnapshotResponse, error) {
	if err := hp.validateGroupControllerServiceRequest(csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT); err != nil {
		klog.V(3).Infof("invalid delete volume group snapshot req: %v", req)
		return nil, err
	}

	// Check arguments
	if len(req.GetGroupSnapshotId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "GroupSnapshot ID missing in request")
	}

	groupSnapshotID := req.GetGroupSnapshotId()

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	groupSnapshot, err := hp.State.GetGroupSnapshotByID(groupSnapshotID)
	if err != nil {
		// ok if NotFound, the VolumeGroupSnapshot was deleted already
		if status.Code(err) == codes.NotFound {
			return &csi.DeleteVolumeGroupSnapshotResponse{}, nil
		}

		return nil, err
	}

	for _, snapshotID := range groupSnapshot.SnapshotIDs {
		klog.V(4).Infof("deleting snapshot %s", snapshotID)
		path := hp.GetSnapshotPath(snapshotID)
		os.RemoveAll(path)

		if err := hp.State.DeleteSnapshot(snapshotID); err != nil {
			return nil, err
		}
	}

	klog.V(4).Infof("deleting groupsnapshot %s", groupSnapshotID)
	if err := hp.State.DeleteGroupSnapshot(groupSnapshotID); err != nil {
		return nil, err
	}

	return &csi.DeleteVolumeGroupSnapshotResponse{}, nil
}

func (hp *GroupControllerServer) GetVolumeGroupSnapshot(ctx context.Context, req *csi.GetVolumeGroupSnapshotRequest) (*csi.GetVolumeGroupSnapshotResponse, error) {
	if err := hp.validateGroupControllerServiceRequest(csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT); err != nil {
		klog.V(3).Infof("invalid get volume group snapshot req: %v", req)
		return nil, err
	}

	groupSnapshotID := req.GetGroupSnapshotId()

	// Check arguments
	if len(groupSnapshotID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "GroupSnapshot ID missing in request")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	groupSnapshot, err := hp.State.GetGroupSnapshotByID(groupSnapshotID)
	if err != nil {
		return nil, err
	}

	if !groupSnapshot.MatchesSnapshotIDs(req.GetSnapshotIds()) {
		return nil, status.Error(codes.InvalidArgument, "Snapshot IDs do not match the GroupSnapshot IDs")
	}

	snapshots := make([]*csi.Snapshot, len(groupSnapshot.SnapshotIDs))
	for i, snapshotID := range groupSnapshot.SnapshotIDs {
		snapshot, err := hp.State.GetSnapshotByID(snapshotID)
		if err != nil {
			return nil, err
		}

		snapshots[i] = &csi.Snapshot{
			SizeBytes:       snapshot.SizeBytes,
			SnapshotId:      snapshotID,
			SourceVolumeId:  snapshot.VolID,
			CreationTime:    snapshot.CreationTime,
			ReadyToUse:      snapshot.ReadyToUse,
			GroupSnapshotId: snapshot.GroupSnapshotID,
		}
	}

	return &csi.GetVolumeGroupSnapshotResponse{
		GroupSnapshot: &csi.VolumeGroupSnapshot{
			GroupSnapshotId: groupSnapshotID,
			Snapshots:       snapshots,
			CreationTime:    groupSnapshot.CreationTime,
			ReadyToUse:      groupSnapshot.ReadyToUse,
		},
	}, nil
}

func (hp *GroupControllerServer) validateGroupControllerServiceRequest(c csi.GroupControllerServiceCapability_RPC_Type) error {
	if c == csi.GroupControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	if c == csi.GroupControllerServiceCapability_RPC_CREATE_DELETE_GET_VOLUME_GROUP_SNAPSHOT {
		return nil
	}

	return status.Errorf(codes.InvalidArgument, "unsupported capability %s", c)
}
