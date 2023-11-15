package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var volumeMap sync.Map

type Volume struct {
	sync.Mutex
	path        string
	attached    bool
	stagePath   string
	publishPath string
	publishNode string
}

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	fmt.Println("CreateVolume:", req)

	ty := req.GetVolumeContentSource().GetType()
	if ty != nil {
		switch ty.(type) {
		case *csi.VolumeContentSource_Snapshot:
			return nil, status.Error(codes.Unimplemented, "Snapshot not supported")
		case *csi.VolumeContentSource_Volume:
			// pass
		default:
			return nil, status.Error(codes.InvalidArgument, "Invalid volume content source type")
		}
	}

	id := uuid.NewString()
	reqVol := req.GetVolumeContentSource().GetVolume()
	if reqVol != nil && reqVol.GetVolumeId() != "" {
		id = reqVol.GetVolumeId()
	}

	path := "/csi/" + req.Name
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	vol := &Volume{
		path:        path,
		attached:    false,
		stagePath:   "",
		publishPath: "",
	}
	fmt.Println("id:", id, "path:", path)
	volumeMap.Store(id, vol)
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      id,
			CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext: req.GetParameters(),
			ContentSource: req.GetVolumeContentSource(),
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	fmt.Println("DeleteVolume:", req)
	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return &csi.DeleteVolumeResponse{}, nil
	}
	vol := volume.(*Volume)
	if err := os.RemoveAll(vol.path); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeMap.Delete(req.VolumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	fmt.Println("ControllerPublishVolume:", req)
	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}
	vol := volume.(*Volume)
	vol.Lock()
	defer vol.Unlock()

	if vol.attached {
		return nil, status.Error(codes.Aborted, "Volume already attached")
	}
	vol.attached = true
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	fmt.Println("ControllerUnpublishVolume:", req)
	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}
	vol := volume.(*Volume)
	vol.Lock()
	defer vol.Unlock()

	vol.attached = false
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	fmt.Println("ValidateVolumeCapabilities:", req)
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				}},
			},
		},
	}, nil
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	fmt.Println("ListVolumes:", req)
	resp := &csi.ListVolumesResponse{}
	var cnt int32 = 0
	volumeMap.Range(func(key, value any) bool {
		volumeId := key.(string)
		volume := value.(*Volume)
		if req.MaxEntries == 0 || cnt < req.MaxEntries {
			var volumePublishedNodes []string
			if volume.publishNode != "" {
				volumePublishedNodes = append(volumePublishedNodes, volume.publishNode)
			}
			resp.Entries = append(resp.Entries, &csi.ListVolumesResponse_Entry{
				Volume: &csi.Volume{
					VolumeId:      volumeId,
					VolumeContext: map[string]string{"path": volume.path},
				},
				Status: &csi.ListVolumesResponse_VolumeStatus{
					PublishedNodeIds: volumePublishedNodes,
				},
			})
			cnt++
		}
		return true
	})
	return resp, nil
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	fmt.Println("GetCapacity:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	fmt.Println("ControllerGetCapabilities:", req)
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	fmt.Println("CreateSnapshot:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	fmt.Println("DeleteSnapshot:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	fmt.Println("ListSnapshots:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	fmt.Println("ControllerExpandVolume:", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	fmt.Println("ControllerGetVolume:", req)
	volume, exist := volumeMap.Load(req.VolumeId)
	if !exist {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}
	vol := volume.(*Volume)
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.VolumeId,
			VolumeContext: map[string]string{"path": vol.path},
		},
	}, nil
}

func (d *Driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	fmt.Println("ControllerModifyVolume:", req)
	return nil, status.Error(codes.Unimplemented, "")
}
