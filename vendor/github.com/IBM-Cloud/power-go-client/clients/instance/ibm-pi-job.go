package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_jobs"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIJobClient
type IBMPIJobClient struct {
	IBMPIClient
}

// NewIBMPIJobClient
func NewIBMPIJobClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIJobClient {
	return &IBMPIJobClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a Job
func (f *IBMPIJobClient) Get(id string) (*models.Job, error) {
	params := p_cloud_jobs.NewPcloudCloudinstancesJobsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithJobID(id)
	resp, err := f.session.Power.PCloudJobs.PcloudCloudinstancesJobsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetJobOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform get Job operation for job id %s", id)
	}
	return resp.Payload, nil
}

// Get All Jobs
func (f *IBMPIJobClient) GetAll() (*models.Jobs, error) {
	params := p_cloud_jobs.NewPcloudCloudinstancesJobsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudJobs.PcloudCloudinstancesJobsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetAllJobsOperationFailed, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to perform get all jobs")
	}
	return resp.Payload, nil
}

// Delete a Job
func (f *IBMPIJobClient) Delete(id string) error {
	params := p_cloud_jobs.NewPcloudCloudinstancesJobsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithJobID(id)
	_, err := f.session.Power.PCloudJobs.PcloudCloudinstancesJobsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeleteJobsOperationFailed, id, err)
	}
	return nil
}
