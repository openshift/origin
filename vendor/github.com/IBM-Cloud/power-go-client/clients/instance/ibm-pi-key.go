package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_tenants_ssh_keys"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIKeyClient
type IBMPIKeyClient struct {
	IBMPIClient
}

// NewIBMPIKeyClient
func NewIBMPIKeyClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIKeyClient {
	return &IBMPIKeyClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a SSH Key
func (f *IBMPIKeyClient) Get(id string) (*models.SSHKey, error) {
	var tenantid = f.session.Options.UserAccount
	params := p_cloud_tenants_ssh_keys.NewPcloudTenantsSshkeysGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithTenantID(tenantid).WithSshkeyName(id)
	resp, err := f.session.Power.PCloudTenantsSSHKeys.PcloudTenantsSshkeysGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetPIKeyOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get PI Key %s", id)
	}
	return resp.Payload, nil
}

// Get All SSH Keys
func (f *IBMPIKeyClient) GetAll() (*models.SSHKeys, error) {
	var tenantid = f.session.Options.UserAccount
	params := p_cloud_tenants_ssh_keys.NewPcloudTenantsSshkeysGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithTenantID(tenantid)
	resp, err := f.session.Power.PCloudTenantsSSHKeys.PcloudTenantsSshkeysGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all PI Keys: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all PI Keys")
	}
	return resp.Payload, nil
}

// Create a SSH Key
func (f *IBMPIKeyClient) Create(body *models.SSHKey) (*models.SSHKey, error) {
	var tenantid = f.session.Options.UserAccount
	params := p_cloud_tenants_ssh_keys.NewPcloudTenantsSshkeysPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithTenantID(tenantid).WithBody(body)
	postok, postcreated, err := f.session.Power.PCloudTenantsSSHKeys.PcloudTenantsSshkeysPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreatePIKeyOperationFailed, err))
	}
	if postok != nil && postok.Payload != nil {
		return postok.Payload, nil
	}
	if postcreated != nil && postcreated.Payload != nil {
		return postcreated.Payload, nil
	}
	return nil, fmt.Errorf("failed to Create PI Key")
}

// Delete a SSH Key
func (f *IBMPIKeyClient) Delete(id string) error {
	var tenantid = f.session.Options.UserAccount
	params := p_cloud_tenants_ssh_keys.NewPcloudTenantsSshkeysDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithTenantID(tenantid).WithSshkeyName(id)
	_, err := f.session.Power.PCloudTenantsSSHKeys.PcloudTenantsSshkeysDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeletePIKeyOperationFailed, id, err)
	}
	return nil
}
