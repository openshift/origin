package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_cloud_connections"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPICloudConnectionClient
type IBMPICloudConnectionClient struct {
	IBMPIClient
}

// NewIBMPICloudConnectionClient
func NewIBMPICloudConnectionClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPICloudConnectionClient {
	return &IBMPICloudConnectionClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Create a Cloud Connection
func (f *IBMPICloudConnectionClient) Create(body *models.CloudConnectionCreate) (*models.CloudConnection, *models.CloudConnectionCreateResponse, error) {
	if f.session.IsOnPrem() {
		return nil, nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	postok, postcreated, postaccepted, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateCloudConnectionOperationFailed, f.cloudInstanceID, err))
	}
	if postok != nil && postok.Payload != nil {
		return postok.Payload, nil, nil
	}
	if postcreated != nil && postcreated.Payload != nil {
		return postcreated.Payload, nil, nil
	}
	if postaccepted != nil && postaccepted.Payload != nil {
		return nil, postaccepted.Payload, nil
	}
	return nil, nil, fmt.Errorf("failed to Create Cloud Connection")
}

// Get a Cloud Connection
func (f *IBMPICloudConnectionClient) Get(id string) (*models.CloudConnection, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloudConnectionID(id)
	resp, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetCloudConnectionOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform Get Cloud Connections Operation for cloudconnectionid %s", id)
	}
	return resp.Payload, nil
}

// Get All Cloud Connections
func (f *IBMPICloudConnectionClient) GetAll() (*models.CloudConnections, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all Cloud Connections: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Cloud Connections")
	}
	return resp.Payload, nil
}

// Update a Cloud Connection
func (f *IBMPICloudConnectionClient) Update(id string, body *models.CloudConnectionUpdate) (*models.CloudConnection, *models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloudConnectionID(id).
		WithBody(body)
	putok, putaccepted, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.UpdateCloudConnectionOperationFailed, id, err))
	}
	if putok != nil && putok.Payload != nil {
		return putok.Payload, nil, nil
	}
	if putaccepted != nil && putaccepted.Payload != nil {
		return nil, putaccepted.Payload, nil
	}
	return nil, nil, fmt.Errorf("failed to Update Cloud Connection %s", id)
}

// Delete a Cloud Connection
func (f *IBMPICloudConnectionClient) Delete(id string) (*models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloudConnectionID(id)
	_, delaccepted, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.DeleteCloudConnectionOperationFailed, id, err))
	}
	if delaccepted != nil && delaccepted.Payload != nil {
		return delaccepted.Payload, nil
	}
	return nil, nil
}

// Add a Network to a Cloud Connection
func (f *IBMPICloudConnectionClient) AddNetwork(id, networkID string) (models.Object, *models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsNetworksPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloudConnectionID(id).
		WithNetworkID(networkID)
	respok, respAccepted, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsNetworksPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Add Network %s to Cloud Connection %s: %w", networkID, id, err))
	}
	if respok != nil && respok.Payload != nil {
		return respok.Payload, nil, nil
	}
	if respAccepted != nil && respAccepted.Payload != nil {
		return nil, respAccepted.Payload, nil
	}
	return nil, nil, nil
}

// Delete a Network from a Cloud Connection
func (f *IBMPICloudConnectionClient) DeleteNetwork(id, networkID string) (models.Object, *models.JobReference, error) {
	if f.session.IsOnPrem() {
		return nil, nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsNetworksDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithCloudConnectionID(id).
		WithNetworkID(networkID)
	respok, respAccepted, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsNetworksDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Delete Network %s from Cloud Connection %s: %w", networkID, id, err))
	}
	if respok != nil && respok.Payload != nil {
		return respok.Payload, nil, nil
	}
	if respAccepted != nil && respAccepted.Payload != nil {
		return nil, respAccepted.Payload, nil
	}
	return nil, nil, nil
}

// Get all VPCs for a Cloud Instance
func (f *IBMPICloudConnectionClient) GetVPC() (*models.CloudConnectionVirtualPrivateClouds, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_cloud_connections.NewPcloudCloudconnectionsVirtualprivatecloudsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)

	resp, err := f.session.Power.PCloudCloudConnections.PcloudCloudconnectionsVirtualprivatecloudsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to perform the get vpc operation: %w", err))
	}
	if resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform the get vpc operation")
	}
	return resp.Payload, nil
}
