package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_shared_processor_pools"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPISharedProcessorPoolClient
type IBMPISharedProcessorPoolClient struct {
	IBMPIClient
}

// NewIBMPISharedProcessorPoolClient
func NewIBMPISharedProcessorPoolClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPISharedProcessorPoolClient {
	return &IBMPISharedProcessorPoolClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a PI Shared Processor Pool
func (f *IBMPISharedProcessorPoolClient) Get(id string) (*models.SharedProcessorPoolDetail, error) {
	params := p_cloud_shared_processor_pools.NewPcloudSharedprocessorpoolsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithSharedProcessorPoolID(id)
	resp, err := f.session.Power.PCloudSharedProcessorPools.PcloudSharedprocessorpoolsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetSharedProcessorPoolOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Shared Processor Pool %s", id)
	}
	return resp.Payload, nil
}

// Get All Shared Processor Pools
func (f *IBMPISharedProcessorPoolClient) GetAll() (*models.SharedProcessorPools, error) {
	params := p_cloud_shared_processor_pools.NewPcloudSharedprocessorpoolsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudSharedProcessorPools.PcloudSharedprocessorpoolsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get All Shared Processor Pools: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Shared Processor Pools")
	}
	return resp.Payload, nil
}

// Create a Shared Processor Pool
func (f *IBMPISharedProcessorPoolClient) Create(body *models.SharedProcessorPoolCreate) (*models.SharedProcessorPool, error) {
	// Check for satellite differences in this endpoint
	if f.session.IsOnPrem() && body.HostID != "" {
		return nil, fmt.Errorf("host id parameter is not supported in on-prem location")
	}
	params := p_cloud_shared_processor_pools.NewPcloudSharedprocessorpoolsPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	postok, err := f.session.Power.PCloudSharedProcessorPools.PcloudSharedprocessorpoolsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateSharedProcessorPoolOperationFailed, f.cloudInstanceID, err))
	}
	if postok == nil || postok.Payload == nil {
		return nil, fmt.Errorf("failed to Create a Shared Processor Pool")
	}
	return postok.Payload, nil
}

// Delete a Shared Processor Pool
func (f *IBMPISharedProcessorPoolClient) Delete(id string) error {
	params := p_cloud_shared_processor_pools.NewPcloudSharedprocessorpoolsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithSharedProcessorPoolID(id)
	_, err := f.session.Power.PCloudSharedProcessorPools.PcloudSharedprocessorpoolsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeleteSharedProcessorPoolOperationFailed, id, err)
	}
	return nil
}

// Update a PI Shared Processor Pool
func (f *IBMPISharedProcessorPoolClient) Update(id string, body *models.SharedProcessorPoolUpdate) (*models.SharedProcessorPool, error) {
	params := p_cloud_shared_processor_pools.NewPcloudSharedprocessorpoolsPutParams().
		WithContext(f.ctx).WithTimeout(helpers.PIUpdateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body).WithSharedProcessorPoolID(id)
	resp, err := f.session.Power.PCloudSharedProcessorPools.PcloudSharedprocessorpoolsPut(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.UpdateSharedProcessorPoolOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Update Shared Processor Pool %s", id)
	}
	return resp.Payload, nil
}
