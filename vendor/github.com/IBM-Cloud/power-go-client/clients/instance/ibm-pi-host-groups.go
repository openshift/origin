package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/host_groups"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIHostGroupsClient
type IBMPIHostGroupsClient struct {
	IBMPIClient
}

// NewIBMPIHostGroupsClient
func NewIBMPIHostGroupsClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIHostGroupsClient {
	return &IBMPIHostGroupsClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get All available hosts
func (f *IBMPIHostGroupsClient) GetAvailableHosts() (models.AvailableHostList, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1AvailableHostsParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.HostGroups.V1AvailableHosts(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get available hosts for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get available hosts")
	}
	return resp.Payload, nil
}

// Get all host groups
func (f *IBMPIHostGroupsClient) GetHostGroups() (models.HostGroupList, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostGroupsGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.HostGroups.V1HostGroupsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get Host groups for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get host groups")
	}
	return resp.Payload, nil
}

// Create a host group
func (f *IBMPIHostGroupsClient) CreateHostGroup(body *models.HostGroupCreate) (*models.HostGroup, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostGroupsPostParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithBody(body)
	resp, err := f.session.Power.HostGroups.V1HostGroupsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to create host group for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to create host groups")
	}
	return resp.Payload, nil
}

// Update a host group
func (f *IBMPIHostGroupsClient) UpdateHostGroup(body *models.HostGroupShareOp, id string) (*models.HostGroup, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostGroupsIDPutParams().WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).WithBody(body).WithHostGroupID(id)
	resp, err := f.session.Power.HostGroups.V1HostGroupsIDPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update host group for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to update host groups")
	}
	return resp.Payload, nil
}

// Get a host group
func (f *IBMPIHostGroupsClient) GetHostGroup(id string) (*models.HostGroup, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostGroupsIDGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithHostGroupID(id)
	resp, err := f.session.Power.HostGroups.V1HostGroupsIDGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get host group %s for %s: %w", id, f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get host group %s", id)
	}
	return resp.Payload, nil
}

// Get  all hosts
func (f *IBMPIHostGroupsClient) GetHosts() (models.HostList, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	hostReference := true
	params := host_groups.NewV1HostsGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	params.HostReference = &hostReference
	resp, err := f.session.Power.HostGroups.V1HostsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get hosts for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get hosts")
	}
	return resp.Payload, nil
}

// Create a host
func (f *IBMPIHostGroupsClient) CreateHost(body *models.HostCreate) (models.HostList, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostsPostParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithBody(body)
	resp, err := f.session.Power.HostGroups.V1HostsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to create a host for %s: %w", f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to create a host")
	}
	return resp.Payload, nil
}

// Get a host
func (f *IBMPIHostGroupsClient) GetHost(id string) (*models.Host, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	hostReference := true
	params := host_groups.NewV1HostsIDGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithHostID(id)
	params.HostReference = &hostReference
	resp, err := f.session.Power.HostGroups.V1HostsIDGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get host %s for %s: %w", id, f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get host %s", id)
	}
	return resp.Payload, nil
}

// Update a host
func (f *IBMPIHostGroupsClient) UpdateHost(body *models.HostPut, id string) (*models.Host, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostsIDPutParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithHostID(id).WithBody(body)
	resp, err := f.session.Power.HostGroups.V1HostsIDPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to update host %s for %s: %w", id, f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to update host %s", id)
	}
	return resp.Payload, nil
}

// Delete a host
func (f *IBMPIHostGroupsClient) DeleteHost(id string) error {
	if f.session.IsOnPrem() {
		return fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := host_groups.NewV1HostsIDDeleteParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithHostID(id)
	resp, err := f.session.Power.HostGroups.V1HostsIDDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to delete host %s for %s: %w", id, f.cloudInstanceID, err))
	}

	if resp == nil || resp.Payload == nil {
		return fmt.Errorf("failed to delete host %s", id)
	}
	return nil
}
