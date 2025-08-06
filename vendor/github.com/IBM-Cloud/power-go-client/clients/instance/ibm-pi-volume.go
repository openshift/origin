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

// IBMPIVolumeClient
type IBMPIVolumeClient struct {
	IBMPIClient
}

// NewIBMPIVolumeClient
func NewIBMPIVolumeClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIVolumeClient {
	return &IBMPIVolumeClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a Volume
func (f *IBMPIVolumeClient) Get(id string) (*models.Volume, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetVolumeOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Volume %s", id)
	}
	return resp.Payload, nil
}

// Get All Volumes
func (f *IBMPIVolumeClient) GetAll() (*models.Volumes, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all Volumes for Cloud Instance %s: %w", f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Volumes for Cloud Instance %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Get All Affinity Volumes
func (f *IBMPIVolumeClient) GetAllAffinityVolumes(affinity string) (*models.Volumes, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithAffinity(&affinity)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all Volumes with affinity %s for Cloud Instance %s: %w", affinity, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Volumes with affinity %s for Cloud Instance %s", affinity, f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Create a VolumeV2
func (f *IBMPIVolumeClient) CreateVolumeV2(body *models.MultiVolumesCreate) (*models.Volumes, error) {
	params := p_cloud_volumes.NewPcloudV2VolumesPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2VolumesPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateVolumeV2OperationFailed, *body.Name, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Create Volume v2")
	}
	return resp.Payload, nil
}

// Create a Volume
func (f *IBMPIVolumeClient) CreateVolume(body *models.CreateDataVolume) (*models.Volume, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateVolumeOperationFailed, *body.Name, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Create Volume")
	}
	return resp.Payload, nil
}

// Update a Volume
func (f *IBMPIVolumeClient) UpdateVolume(id string, body *models.UpdateVolume) (*models.Volume, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id).
		WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.UpdateVolumeOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Update Volume %s", id)
	}
	return resp.Payload, nil
}

// Delete a Volume
func (f *IBMPIVolumeClient) DeleteVolume(id string) error {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id)
	_, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeleteVolumeOperationFailed, id, err)
	}
	return nil
}

// Attach a Volume to an Instance
func (f *IBMPIVolumeClient) Attach(id, volumename string) error {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id).
		WithVolumeID(volumename)
	_, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.AttachVolumeOperationFailed, volumename, err)
	}
	return nil
}

// Detach a Volume from an Instance
func (f *IBMPIVolumeClient) Detach(id, volumename string) error {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id).
		WithVolumeID(volumename)
	_, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DetachVolumeOperationFailed, volumename, err)
	}
	return nil
}

// Get All Volumes attached to an Instance
func (f *IBMPIVolumeClient) GetAllInstanceVolumes(id string) (*models.Volumes, error) {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id)
	resp, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all Volumes for PI Instance %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Volumes for PI Instance %s", id)
	}
	return resp.Payload, nil
}

// Set a Volume as the Boot Volume for an Instance
func (f *IBMPIVolumeClient) SetBootVolume(id, volumename string) error {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesSetbootPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id).
		WithVolumeID(volumename)
	_, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesSetbootPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to set the boot volume %s for instance %s", volumename, id)
	}
	return nil
}

// Check if a Volume is attached to an Instance
func (f *IBMPIVolumeClient) CheckVolumeAttach(id, volumeID string) (*models.Volume, error) {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id).
		WithVolumeID(volumeID)
	resp, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to validate that the volume %s is attached to the pvminstance %s: %w", volumeID, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to validate that the volume %s is attached to the pvminstance %s", volumeID, id)
	}
	return resp.Payload, nil
}

// Update a Volume attached to an Instance
func (f *IBMPIVolumeClient) UpdateVolumeAttach(id, volumeID string, body *models.PVMInstanceVolumeUpdate) error {
	params := p_cloud_volumes.NewPcloudPvminstancesVolumesPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(id).
		WithVolumeID(volumeID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudPvminstancesVolumesPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to validate that the volume %s is attached to the pvminstance %s: %w", volumeID, id, err)
	}
	if resp == nil || resp.Payload == nil {
		return fmt.Errorf("failed to validate that the volume %s is attached to the pvminstance %s", volumeID, id)
	}
	return nil
}

// Performs action on volume
func (f *IBMPIVolumeClient) VolumeAction(id string, body *models.VolumeAction) error {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesActionPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id).WithBody(body)
	_, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesActionPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to perform action on volume %s with error %w", id, err)
	}
	return nil
}

// Get remote copy relationship of a volume
func (f *IBMPIVolumeClient) GetVolumeRemoteCopyRelationships(id string) (*models.VolumeRemoteCopyRelationship, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesRemoteCopyRelationshipGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesRemoteCopyRelationshipGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetVolumeRemoteCopyRelationshipsOperationFailed, id, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get remote copy relationships of a volume %s", id)
	}
	return resp.Payload, nil
}

// Get a list of flashcopy mappings of a given volume
func (f *IBMPIVolumeClient) GetVolumeFlashCopyMappings(id string) (models.FlashCopyMappings, error) {
	params := p_cloud_volumes.NewPcloudCloudinstancesVolumesFlashCopyMappingsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeID(id)
	resp, err := f.session.Power.PCloudVolumes.PcloudCloudinstancesVolumesFlashCopyMappingsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetVolumeFlashCopyMappingOperationFailed, id, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get flash copy mapping of a volume %s", id)
	}
	return resp.Payload, nil
}

// Bulk volume detach
func (f *IBMPIVolumeClient) BulkVolumeDetach(pvmID string, body *models.VolumesDetach) (*models.VolumesDetachmentResponse, error) {
	params := p_cloud_volumes.NewPcloudV2PvminstancesVolumesDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(pvmID).
		WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2PvminstancesVolumesDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.DetachVolumesOperationFailed, pvmID, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed detaching volumes for %s", pvmID)
	}
	return resp.Payload, nil
}

// Bulk volume delete
func (f *IBMPIVolumeClient) BulkVolumeDelete(body *models.VolumesDelete) (*models.VolumesDeleteResponse, error) {
	params := p_cloud_volumes.NewPcloudV2VolumesDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	respOk, respPartial, err := f.session.Power.PCloudVolumes.PcloudV2VolumesDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.DeleteVolumeOperationFailed, body.VolumeIDs, err))
	}
	if respOk != nil && respOk.Payload != nil {
		return respOk.Payload, nil
	}
	if respPartial != nil && respPartial.Payload != nil {
		return respPartial.Payload, nil
	}
	return nil, fmt.Errorf("failed Deleting volumes : %s", body.VolumeIDs)
}

// Bulk volutme attach
func (f *IBMPIVolumeClient) BulkVolumeAttach(pvmID string, body *models.VolumesAttach) (*models.VolumesAttachmentResponse, error) {
	params := p_cloud_volumes.NewPcloudV2PvminstancesVolumesPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(pvmID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumes.PcloudV2PvminstancesVolumesPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.AttachVolumesOperationFailed, body.VolumeIDs, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Attach volumes %s to server %s", body.VolumeIDs, pvmID)
	}
	return resp.Payload, nil
}
