package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/ssh_keys"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPISSHKeyClient
type IBMPISSHKeyClient struct {
	IBMPIClient
}

// NewIBMPISSHKeyClient
func NewIBMPISSHKeyClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPISSHKeyClient {
	return &IBMPISSHKeyClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a SSH Key
func (f *IBMPISSHKeyClient) Get(id string) (*models.WorkspaceSSHKey, error) {
	params := ssh_keys.NewV1SshkeysGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithSshkeyID(id)

	resp, err := f.session.Power.SSHKeys.V1SshkeysGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetPISSHKeyOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get PI SSH Key %s", id)
	}
	return resp.Payload, nil
}

// Get All SSH Keys
func (f *IBMPISSHKeyClient) GetAll() (*models.WorkspaceSSHKeys, error) {
	params := ssh_keys.NewV1SshkeysGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)

	resp, err := f.session.Power.SSHKeys.V1SshkeysGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetAllPISSHKeyOperationFailed, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all PI SSH Keys")
	}
	return resp.Payload, nil
}

// Create a SSH Key
func (f *IBMPISSHKeyClient) Create(body *models.CreateWorkspaceSSHKey) (*models.WorkspaceSSHKey, error) {
	params := ssh_keys.NewV1SshkeysPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithBody(body)
	resp, err := f.session.Power.SSHKeys.V1SshkeysPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreatePISSHKeyOperationFailed, err))
	}
	if resp != nil && resp.Payload != nil {
		return resp.Payload, nil
	}
	return nil, fmt.Errorf("failed to Create PI SSH Key")
}

// Delete a SSH Key
func (f *IBMPISSHKeyClient) Delete(id string) error {
	params := ssh_keys.NewV1SshkeysDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithSshkeyID(id)
	_, err := f.session.Power.SSHKeys.V1SshkeysDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeletePISSHKeyOperationFailed, id, err)
	}
	return nil
}

// Update an SSH Key
func (f *IBMPISSHKeyClient) Update(id string, body *models.UpdateWorkspaceSSHKey) (*models.WorkspaceSSHKey, error) {
	params := ssh_keys.NewV1SshkeysPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithSshkeyID(id).WithBody(body)

	resp, err := f.session.Power.SSHKeys.V1SshkeysPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.UpdatePISSHKeyOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to update ssh key %s", id)
	}
	return resp.Payload, nil
}
