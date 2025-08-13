package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_tasks"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPITaskClient
type IBMPITaskClient struct {
	IBMPIClient
}

// NewIBMPITaskClient
func NewIBMPITaskClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPITaskClient {
	return &IBMPITaskClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a Task
func (f *IBMPITaskClient) Get(id string) (*models.Task, error) {
	params := p_cloud_tasks.NewPcloudTasksGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithTaskID(id)
	resp, err := f.session.Power.PCloudTasks.PcloudTasksGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get the task %s: %w", id, err)
	}
	return resp.Payload, nil
}

// Delete a Task
func (f *IBMPITaskClient) Delete(id string) error {
	params := p_cloud_tasks.NewPcloudTasksDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithTaskID(id)
	_, err := f.session.Power.PCloudTasks.PcloudTasksDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf("failed to delete the task id ... %w", err)
	}
	return nil
}
