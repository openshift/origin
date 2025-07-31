package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/network_address_groups"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPINetworkAddressGroupClient
type IBMPINetworkAddressGroupClient struct {
	IBMPIClient
}

// NewIBMPINetworkAddressGroupClient
func NewIBMPINetworkAddressGroupClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPINetworkAddressGroupClient {
	return &IBMPINetworkAddressGroupClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Create a new Network Address Group
func (f *IBMPINetworkAddressGroupClient) Create(body *models.NetworkAddressGroupCreate) (*models.NetworkAddressGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithBody(body)
	postok, postcreated, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to create a network address group %s", err))
	}
	if postok != nil && postok.Payload != nil {
		return postok.Payload, nil
	}
	if postcreated != nil && postcreated.Payload != nil {
		return postcreated.Payload, nil
	}
	return nil, fmt.Errorf("failed to create a network address group")
}

// Get the list of Network Address Groups for a workspace
func (f *IBMPINetworkAddressGroupClient) GetAll() (*models.NetworkAddressGroups, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get network address groups %s", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get network address groups")
	}
	return resp.Payload, nil

}

// Get the detail of a Network Address Group
func (f *IBMPINetworkAddressGroupClient) Get(id string) (*models.NetworkAddressGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsIDGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithNetworkAddressGroupID(id)
	resp, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsIDGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get network address group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get network address group %s", id)
	}
	return resp.Payload, nil

}

// Update a Network Address Group
func (f *IBMPINetworkAddressGroupClient) Update(id string, body *models.NetworkAddressGroupUpdate) (*models.NetworkAddressGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsIDPutParams().WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).WithNetworkAddressGroupID(id).WithBody(body)
	resp, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsIDPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update network address group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to update network address group %s", id)
	}
	return resp.Payload, nil
}

// Delete a Network Address Group from a workspace
func (f *IBMPINetworkAddressGroupClient) Delete(id string) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsIDDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithNetworkAddressGroupID(id)
	_, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsIDDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to delete network address group %s: %w", id, err)
	}
	return nil
}

// Add a member to a Network Address Group
func (f *IBMPINetworkAddressGroupClient) AddMember(id string, body *models.NetworkAddressGroupAddMember) (*models.NetworkAddressGroupMember, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsMembersPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithNetworkAddressGroupID(id).WithBody(body)
	resp, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsMembersPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to add member to network address group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to add member to network address group %s", id)
	}
	return resp.Payload, nil
}

// Delete the member from a Network Address Group
func (f *IBMPINetworkAddressGroupClient) DeleteMember(id, memberId string) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_address_groups.NewV1NetworkAddressGroupsMembersDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithNetworkAddressGroupID(id).WithNetworkAddressGroupMemberID(memberId)
	_, err := f.session.Power.NetworkAddressGroups.V1NetworkAddressGroupsMembersDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete member %s from network address group %s: %w", memberId, id, err))
	}

	return nil
}
