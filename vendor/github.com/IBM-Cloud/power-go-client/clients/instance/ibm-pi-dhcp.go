package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_service_d_h_c_p"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// NewIBMPIDhcpClient
type IBMPIDhcpClient struct {
	IBMPIClient
}

// NewIBMPIDhcpClient
func NewIBMPIDhcpClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIDhcpClient {
	return &IBMPIDhcpClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Create a DHCP server
func (f *IBMPIDhcpClient) Create(body *models.DHCPServerCreate) (*models.DHCPServer, error) {
	params := p_cloud_service_d_h_c_p.NewPcloudDhcpPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	postaccepted, err := f.session.Power.PCloudServicedhcp.PcloudDhcpPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateDchpOperationFailed, f.cloudInstanceID, err))
	}
	if postaccepted != nil && postaccepted.Payload != nil {
		return postaccepted.Payload, nil
	}
	return nil, fmt.Errorf("failed to Create DHCP")
}

// Get a DHCP server
func (f *IBMPIDhcpClient) Get(id string) (*models.DHCPServerDetail, error) {
	params := p_cloud_service_d_h_c_p.NewPcloudDhcpGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithDhcpID(id)
	resp, err := f.session.Power.PCloudServicedhcp.PcloudDhcpGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetDhcpOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get DHCP %s", id)
	}
	return resp.Payload, nil
}

// Get All DHCP servers
func (f *IBMPIDhcpClient) GetAll() (models.DHCPServers, error) {
	params := p_cloud_service_d_h_c_p.NewPcloudDhcpGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudServicedhcp.PcloudDhcpGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all DHCP servers: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all DHCP servers")
	}
	return resp.Payload, nil
}

// Delete a DHCP server
func (f *IBMPIDhcpClient) Delete(id string) error {
	params := p_cloud_service_d_h_c_p.NewPcloudDhcpDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithDhcpID(id)
	_, err := f.session.Power.PCloudServicedhcp.PcloudDhcpDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeleteDhcpOperationFailed, id, err)
	}
	return nil
}
