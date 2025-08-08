package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_v_p_n_connections"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIVpnConnectionClient
type IBMPIVpnConnectionClient struct {
	IBMPIClient
}

// NewIBMPIVpnConnectionClient
func NewIBMPIVpnConnectionClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIVpnConnectionClient {
	return &IBMPIVpnConnectionClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Deprecated Get a VPN Connection
func (f *IBMPIVpnConnectionClient) Get(id string) (*models.VPNConnection, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id)
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetVPNConnectionOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get VPN Connection %s", id)
	}
	return resp.Payload, nil
}

// Deprecated Create a VPN Connection
func (f *IBMPIVpnConnectionClient) Create(body *models.VPNConnectionCreate) (*models.VPNConnectionCreateResponse, error) {
	return nil, fmt.Errorf("Create VPN Connection is no longer supported")
}

// Deprecated Update a VPN Connection
func (f *IBMPIVpnConnectionClient) Update(id string, body *models.VPNConnectionUpdate) (*models.VPNConnection, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id).
		WithBody(body)
	putok, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.UpdateVPNConnectionOperationFailed, id, err))
	}
	if putok != nil && putok.Payload != nil {
		return putok.Payload, nil
	}
	return nil, fmt.Errorf("failed to Update VPN Connection %s", id)
}

// Deprecated Get All VPN Connections
func (f *IBMPIVpnConnectionClient) GetAll() (*models.VPNConnections, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all VPN Connections: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all VPN Connections")
	}
	return resp.Payload, nil
}

// Deprecated Delete a VPN Connection
func (f *IBMPIVpnConnectionClient) Delete(id string) (*models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id)
	delaccepted, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.DeleteVPNConnectionOperationFailed, id, err))
	}
	if delaccepted != nil && delaccepted.Payload != nil {
		return delaccepted.Payload, nil
	}
	return nil, nil
}

// Deprecated Get a VPN Connection's Network
func (f *IBMPIVpnConnectionClient) GetNetwork(id string) (*models.NetworkIDs, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsNetworksGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id)
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsNetworksGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get Networks for VPN Connection %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Networks for VPN Connection %s", id)
	}
	return resp.Payload, nil
}

// Deprecated Attach a Network to a VPN Connection
func (f *IBMPIVpnConnectionClient) AddNetwork(id, networkID string) (*models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsNetworksPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id).
		WithBody(&models.NetworkID{NetworkID: &networkID})
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsNetworksPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Add Network %s to VPN Connection %s: %w", networkID, id, err))
	}
	if resp != nil && resp.Payload != nil {
		return resp.Payload, nil
	}
	return nil, nil
}

// Deprecated Detach a Network from a VPN Connection
func (f *IBMPIVpnConnectionClient) DeleteNetwork(id, networkID string) (*models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsNetworksDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id).
		WithBody(&models.NetworkID{NetworkID: &networkID})
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsNetworksDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Delete Network %s from VPN Connection %s: %w", networkID, id, err))
	}
	if resp != nil && resp.Payload != nil {
		return resp.Payload, nil
	}
	return nil, nil
}

// Deprecated Get a VPN Connection's Subnet
func (f *IBMPIVpnConnectionClient) GetSubnet(id string) (*models.PeerSubnets, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsPeersubnetsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id)
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsPeersubnetsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get Subnets from VPN Connection %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Subnets from VPN Connection %s", id)
	}
	return resp.Payload, nil
}

// Deprecated Attach a Subnet to a VPN Connection
func (f *IBMPIVpnConnectionClient) AddSubnet(id, subnet string) (*models.PeerSubnets, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsPeersubnetsPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id).
		WithBody(&models.PeerSubnetUpdate{Cidr: &subnet})
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsPeersubnetsPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Add Subnets to VPN Connection %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Add Subnets to VPN Connection %s", id)
	}
	return resp.Payload, nil
}

// Deprecated Detach a Subnet from a VPN Connection
func (f *IBMPIVpnConnectionClient) DeleteSubnet(id, subnet string) (*models.PeerSubnets, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_v_p_n_connections.NewPcloudVpnconnectionsPeersubnetsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVpnConnectionID(id).
		WithBody(&models.PeerSubnetUpdate{Cidr: &subnet})
	resp, err := f.session.Power.PCloudvpnConnections.PcloudVpnconnectionsPeersubnetsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Delete Subnet from VPN Connection %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Delete Subnet from VPN Connection %s", id)
	}
	return resp.Payload, nil
}
