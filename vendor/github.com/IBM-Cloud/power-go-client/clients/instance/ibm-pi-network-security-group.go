package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/network_security_groups"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPINetworkSecurityGroupClient
type IBMPINetworkSecurityGroupClient struct {
	IBMPIClient
}

// NewIBMIPINetworkSecurityGroupClient
func NewIBMIPINetworkSecurityGroupClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPINetworkSecurityGroupClient {
	return &IBMPINetworkSecurityGroupClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a network security group
func (f *IBMPINetworkSecurityGroupClient) Get(id string) (*models.NetworkSecurityGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsIDGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithNetworkSecurityGroupID(id)
	resp, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsIDGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get network security group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get network security group %s", id)
	}
	return resp.Payload, nil
}

// Get all network security groups
func (f *IBMPINetworkSecurityGroupClient) GetAll() (*models.NetworkSecurityGroups, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsListParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsList(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get network security groups %s", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get network security groups")
	}
	return resp.Payload, nil
}

// Create a network security group
func (f *IBMPINetworkSecurityGroupClient) Create(body *models.NetworkSecurityGroupCreate) (*models.NetworkSecurityGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithBody(body)
	postok, postcreated, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to create a network security group %s", err))
	}
	if postok != nil && postok.Payload != nil {
		return postok.Payload, nil
	}
	if postcreated != nil && postcreated.Payload != nil {
		return postcreated.Payload, nil
	}
	return nil, fmt.Errorf("failed to create a network security group")
}

// Update a network security group
func (f *IBMPINetworkSecurityGroupClient) Update(id string, body *models.NetworkSecurityGroupUpdate) (*models.NetworkSecurityGroup, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsIDPutParams().WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).WithNetworkSecurityGroupID(id).WithBody(body)
	resp, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsIDPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update network security group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to update network security group %s", id)
	}
	return resp.Payload, nil
}

// Delete a network security group
func (f *IBMPINetworkSecurityGroupClient) Delete(id string) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsIDDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithNetworkSecurityGroupID(id)
	_, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsIDDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to delete network security group %s: %w", id, err)
	}
	return nil
}

// Add a member to a network security group
func (f *IBMPINetworkSecurityGroupClient) AddMember(id string, body *models.NetworkSecurityGroupAddMember) (*models.NetworkSecurityGroupMember, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsMembersPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithNetworkSecurityGroupID(id).WithBody(body)
	resp, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsMembersPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to add member to network security group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to add member to network security group %s", id)
	}
	return resp.Payload, nil
}

// Deleta a member from a network securti group
func (f *IBMPINetworkSecurityGroupClient) DeleteMember(id, memberId string) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsMembersDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithNetworkSecurityGroupID(id).WithNetworkSecurityGroupMemberID(memberId)
	_, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsMembersDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete member %s from network security group %s: %w", memberId, id, err))
	}
	return nil
}

// Add a rule to a network security group
func (f *IBMPINetworkSecurityGroupClient) AddRule(id string, body *models.NetworkSecurityGroupAddRule) (*models.NetworkSecurityGroupRule, error) {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsRulesPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithNetworkSecurityGroupID(id).WithBody(body)
	resp, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsRulesPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to add rule to network security group %s: %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to add rule to network security group %s", id)
	}
	return resp.Payload, nil
}

// Delete a rule from a network security group
func (f *IBMPINetworkSecurityGroupClient) DeleteRule(id, ruleId string) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsRulesDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).WithNetworkSecurityGroupID(id).WithNetworkSecurityGroupRuleID(ruleId)
	_, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsRulesDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete rule %s from network security group %s: %w", ruleId, id, err))
	}
	return nil
}

// Action on a network security group
func (f *IBMPINetworkSecurityGroupClient) Action(body *models.NetworkSecurityGroupsAction) error {
	// Add check for on-prem location
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := network_security_groups.NewV1NetworkSecurityGroupsActionPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithBody(body)
	_, _, err := f.session.Power.NetworkSecurityGroups.V1NetworkSecurityGroupsActionPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to perform action :%w", err)
	}
	return nil
}
