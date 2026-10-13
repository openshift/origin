package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_virtual_serial_number"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIVSNClient

type IBMPIVSNClient struct {
	IBMPIClient
}

// NewIBMPIVSNClient
func NewIBMPIVSNClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIVSNClient {
	return &IBMPIVSNClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get Virtual Serial Number
func (f *IBMPIVSNClient) Get(id string) (*models.VirtualSerialNumber, error) {
	params := p_cloud_virtual_serial_number.NewPcloudVirtualserialnumberGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithVirtualSerialNumber(id)
	resp, err := f.session.Power.PCloudVirtualSerialNumber.PcloudVirtualserialnumberGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get virtual serial number %s :%w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get virtual serial number %s", id)
	}
	return resp.Payload, nil
}

// Get All Virtual Serial Numbers
func (f *IBMPIVSNClient) GetAll(pvmInstanceID *string) (models.VirtualSerialNumberList, error) {
	params := p_cloud_virtual_serial_number.NewPcloudVirtualserialnumberGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	if pvmInstanceID != nil {
		params.SetPvmInstanceID(pvmInstanceID)
	}
	resp, err := f.session.Power.PCloudVirtualSerialNumber.PcloudVirtualserialnumberGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get all virtual serial numbers in %s :%w", f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get all virtual serial numbers in %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Get All Supported IBMi Software Tiers
func (f *IBMPIVSNClient) GetAllSoftwareTiers() (models.SupportedSoftwareTierList, error) {
	params := p_cloud_virtual_serial_number.NewPcloudVirtualserialnumberSoftwaretiersGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.PCloudVirtualSerialNumber.PcloudVirtualserialnumberSoftwaretiersGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get all supported IBMi software tiers in %s :%w", f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get all supported IBMi software tiers in %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Update Virtual Serial Number
func (f *IBMPIVSNClient) Update(id string, body *models.UpdateVirtualSerialNumber) (*models.GetServerVirtualSerialNumber, error) {
	params := p_cloud_virtual_serial_number.NewPcloudVirtualserialnumberPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).WithVirtualSerialNumber(id).
		WithBody(body)
	resp, err := f.session.Power.PCloudVirtualSerialNumber.PcloudVirtualserialnumberPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update virtual serial number %s :%w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Update virtual serial number %s", id)
	}
	return resp.Payload, nil
}

// Delete Virtual Serial Number
func (f *IBMPIVSNClient) Delete(id string) error {
	params := p_cloud_virtual_serial_number.NewPcloudVirtualserialnumberDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithVirtualSerialNumber(id)
	_, err := f.session.Power.PCloudVirtualSerialNumber.PcloudVirtualserialnumberDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete virtual serial number %s :%w", id, err))
	}
	return nil
}

// PVM Instance Delete VSN
func (f *IBMPIVSNClient) PVMInstanceDeleteVSN(pvmInstanceID string, body *models.DeleteServerVirtualSerialNumber) error {
	params := p_cloud_virtual_serial_number.NewPcloudPvminstancesVirtualserialnumberDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithCloudInstanceID(f.cloudInstanceID).WithPvmInstanceID(pvmInstanceID).
		WithBody(body)
	_, err := f.session.Power.PCloudVirtualSerialNumber.PcloudPvminstancesVirtualserialnumberDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete virtual serial number for pvm instance %s :%w", pvmInstanceID, err))
	}
	return nil
}

// PVM Instance Update VSN
func (f *IBMPIVSNClient) PVMInstanceUpdateVSN(pvmInstanceID string, body *models.UpdateServerVirtualSerialNumber) (*models.VirtualSerialNumber, error) {
	params := p_cloud_virtual_serial_number.NewPcloudPvminstancesVirtualserialnumberPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithCloudInstanceID(f.cloudInstanceID).
		WithPvmInstanceID(pvmInstanceID).WithBody(body)
	respOk, respAccepted, err := f.session.Power.PCloudVirtualSerialNumber.PcloudPvminstancesVirtualserialnumberPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update virtual serial number for pvm instance %s :%w", pvmInstanceID, err))
	}
	if respOk != nil && respOk.Payload != nil {
		return respOk.Payload, nil
	}
	if respAccepted != nil && respAccepted.Payload != nil {
		return respAccepted.Payload, nil
	}
	return nil, fmt.Errorf("failed to update virtual serial number for pvm instance %s", pvmInstanceID)
}

// PVM Attach VSN
func (f *IBMPIVSNClient) PVMInstanceAttachVSN(pvmInstanceID string, body *models.AddServerVirtualSerialNumber) (*models.VirtualSerialNumber, error) {
	params := p_cloud_virtual_serial_number.NewPcloudPvminstancesVirtualserialnumberPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithCloudInstanceID(f.cloudInstanceID).
		WithPvmInstanceID(pvmInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVirtualSerialNumber.PcloudPvminstancesVirtualserialnumberPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to attach virtual serial number for pvm instance %s :%w", pvmInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to attach virtual serial number for pvm instance %s", pvmInstanceID)
	}
	return resp.Payload, nil
}
