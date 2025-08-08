package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_system_pools"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPISystemPoolClient
type IBMPISystemPoolClient struct {
	IBMPIClient
}

// NewIBMPISystemPoolClient
func NewIBMPISystemPoolClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPISystemPoolClient {
	return &IBMPISystemPoolClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get the System Pools
// Deprecated: Use GetSystemPools()
func (f *IBMPISystemPoolClient) Get(id string) (models.SystemPools, error) {
	params := p_cloud_system_pools.NewPcloudSystempoolsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(id)
	resp, err := f.session.Power.PCloudSystemPools.PcloudSystempoolsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetSystemPoolsOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform Get System Pools Operation for cloud instance id %s", id)
	}
	return resp.Payload, nil
}

// Get the System Pools
func (f *IBMPISystemPoolClient) GetSystemPools() (models.SystemPools, error) {
	params := p_cloud_system_pools.NewPcloudSystempoolsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudSystemPools.PcloudSystempoolsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetSystemPoolsOperationFailed, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform Get System Pools Operation for cloud instance id %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}
