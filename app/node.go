package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	fmt.Println("NodeStageVolume:", req)
	d.Lock()
	defer d.Unlock()

	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}
	vol := volume.(*Volume)
	if !vol.attached {
		return nil, status.Error(codes.FailedPrecondition, "Volume not attached")
	}
	if vol.stagePath != "" {
		return nil, status.Error(codes.AlreadyExists, "Volume already staged")
	}

	vol.stagePath = req.StagingTargetPath
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	fmt.Println("NodeUnstageVolume:", req)
	d.Lock()
	defer d.Unlock()

	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return &csi.NodeUnstageVolumeResponse{}, nil
	}
	vol := volume.(*Volume)
	vol.stagePath = ""
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	fmt.Println("NodeExpandVolume:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	fmt.Println("NodeGetCapabilities:", req)
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	fmt.Println("NodeGetInfo:", req)
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeId,
	}, nil
}

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	fmt.Println("NodeGetVolumeStats:", req)
	d.Lock()
	defer d.Unlock()

	_, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	return &csi.NodeGetVolumeStatsResponse{
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: false,
			Message:  "",
		},
	}, nil
}

func (d *Driver) getPath(volumeId string) string {
	return filepath.Join(d.dataDir, volumeId)
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	fmt.Println("NodePublishVolume:", req)
	switch req.GetVolumeCapability().GetAccessType().(type) {
	case *csi.VolumeCapability_Mount:
	case *csi.VolumeCapability_Block:
		return nil, status.Error(codes.Unimplemented, "Unsupported access type")
	default:
		return nil, status.Error(codes.InvalidArgument, "Unknown access type")
	}

	d.Lock()
	defer d.Unlock()

	// volume, exist := volumeMap.Load(req.VolumeId)
	// if !exist {
	// 	return nil, status.Error(codes.NotFound, "Volume not found")
	// }
	// vol := volume.(*Volume)
	// if vol.publishPath != "" {
	// 	return nil, status.Error(codes.AlreadyExists, "Volume already published")
	// }
	// if vol.stagePath != req.StagingTargetPath {
	// 	return nil, status.Error(codes.InvalidArgument, "StagingTargetPath does not match")
	// }

	path := d.getPath(req.VolumeId)
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	vol := &Volume{
		path:        path,
		attached:    true,
		stagePath:   path,
		publishPath: "",
	}
	volumeMap.Store(req.VolumeId, vol)

	mounter := mount.New("")
	notMount, err := mount.IsNotMountPoint(mounter, req.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(req.TargetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMount = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if !notMount {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	mnt := req.GetVolumeCapability().GetMount()
	options := append(mnt.MountFlags, "bind")
	if err = mounter.Mount(vol.path, req.TargetPath, mnt.FsType, options); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	vol.publishPath = req.TargetPath
	vol.publishNode = d.nodeId

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	fmt.Println("NodeUnpublishVolume:", req)
	d.Lock()
	defer d.Unlock()

	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}
	vol := volume.(*Volume)

	if vol.publishPath == "" || vol.publishPath != req.TargetPath {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	mounter := mount.New("")
	notMount, err := mount.IsNotMountPoint(mounter, req.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("error checking path %s for mount: %w", req.TargetPath, err)
	}
	if !notMount {
		if err = mounter.Unmount(req.TargetPath); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// mount dir
	if err = os.RemoveAll(req.TargetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// data dir
	if err = os.RemoveAll(vol.path); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	vol.publishPath = ""
	vol.publishNode = ""

	return &csi.NodeUnpublishVolumeResponse{}, nil
}
