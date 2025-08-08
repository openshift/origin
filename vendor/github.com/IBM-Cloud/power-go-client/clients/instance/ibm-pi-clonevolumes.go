package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_volumes"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPICloneVolumeClient
type IBMPICloneVolumeClient struct {
	IBMPIClient
}

// NewIBMPICloneVolumeClient
func NewIBMPICloneVolumeClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPICloneVolumeClient {
	return &IBMPICloneVolumeClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Create a Clone Volume (V2) - This creates a clone
func (f *IBMPICloneVolumeClient) Create(body *models.VolumesCloneAsyncRequest) (*models.CloneTaskReference, error) {
	params := p_cloud_volumes.NewPcloudV2VolumesClonePostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumesClonePost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateCloneOperationFailed, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform the create clone operation")
	}
	return resp.Payload, nil
}

// Get Status of a Clone Request
func (f *IBMPICloneVolumeClient) Get(cloneTaskID string) (*models.CloneTaskStatus, error) {
	params := p_cloud_volumes.NewPcloudV2VolumesClonetasksGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloneTaskID(cloneTaskID)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumesClonetasksGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get the clone task %s status for the cloud instance %s with error %w", cloneTaskID, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get the clone task %s status for the cloud instance %s", cloneTaskID, f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Create a Clone Volume (V2) - This is the prepare operation
func (f *IBMPICloneVolumeClient) CreateV2Clone(body *models.VolumesCloneCreate) (*models.VolumesClone, error) {
	params := p_cloud_volumes.NewPcloudV2VolumesclonePostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumesclonePost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.PrepareCloneOperationFailed, *body.Name, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to prepare the clone operation")
	}
	return resp.Payload, nil
}

// Get a list of Volume Clones
func (f *IBMPICloneVolumeClient) GetV2Clones(queryFilter string) (*models.VolumesClones, error) {
	params := p_cloud_volumes.NewPcloudV2VolumescloneGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithFilter(&queryFilter)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumescloneGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get the volumes-clones for the cloud instance %s with error %w", f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get the volumes-clones for the cloud instance %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Delete a Volume Clone
func (f *IBMPICloneVolumeClient) DeleteClone(id string) error {
	params := p_cloud_volumes.NewPcloudV2VolumescloneDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumesCloneID(id)
	_, err := f.session.Power.PCloudVolumes.PcloudV2VolumescloneDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeleteCloneOperationFailed, err)
	}
	return nil
}

// Initiate a Start Clone Request
func (f *IBMPICloneVolumeClient) StartClone(volumesCloneID string) (*models.VolumesClone, error) {
	params := p_cloud_volumes.NewPcloudV2VolumescloneStartPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumesCloneID(volumesCloneID)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumescloneStartPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.StartCloneOperationFailed, volumesCloneID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to start the clone operation for volume-clone %s", volumesCloneID)
	}
	return resp.Payload, nil
}

// Initiate an Execute Action for a Clone
func (f *IBMPICloneVolumeClient) PrepareClone(volumesCloneID string) (*models.VolumesClone, error) {
	params := p_cloud_volumes.NewPcloudV2VolumescloneExecutePostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumesCloneID(volumesCloneID)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumescloneExecutePost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.PrepareCloneOperationFailed, volumesCloneID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to prepare the clone operation for %s", volumesCloneID)
	}
	return resp.Payload, nil
}

// Get a V2Clone Task Status
func (f *IBMPICloneVolumeClient) GetV2CloneStatus(cloneName string) (*models.VolumesCloneDetail, error) {
	params := p_cloud_volumes.NewPcloudV2VolumescloneGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumesCloneID(cloneName)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumescloneGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetCloneOperationFailed, cloneName, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get the volumes-clone %s for the cloud instance %s", cloneName, f.cloudInstanceID)
	}
	return resp.Payload, nil
}
