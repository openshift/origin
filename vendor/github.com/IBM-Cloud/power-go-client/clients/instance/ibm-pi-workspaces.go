package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/workspaces"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
)

// IBMPIWorkspacesClient
type IBMPIWorkspacesClient struct {
	IBMPIClient
}

// Convert into valid plan or return empty (resulting in error)
func translatePlan(plan string) string {
	planID := ""
	switch plan {
	case "public":
		planID = "f165dd34-3a40-423b-9d95-e90a23f724dd"
	case "private":
		planID = "1112d6a9-71d6-4968-956b-eb3edbf0225b"
	default:
		planID = ""
	}
	return planID
}

// NewIBMPIWorkspacesClient
func NewIBMPIWorkspacesClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIWorkspacesClient {
	return &IBMPIWorkspacesClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a workspace
func (f *IBMPIWorkspacesClient) Get(cloudInstanceID string) (*models.Workspace, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := workspaces.NewV1WorkspacesGetParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).WithWorkspaceID(cloudInstanceID)
	resp, err := f.session.Power.Workspaces.V1WorkspacesGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetWorkspaceOperationFailed, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Workspace %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Get all workspaces
func (f *IBMPIWorkspacesClient) GetAll() (*models.Workspaces, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := workspaces.NewV1WorkspacesGetallParams().WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.Workspaces.V1WorkspacesGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get all Workspaces: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Workspaces")
	}
	return resp.Payload, nil
}

// Create a workspace
func (f *IBMPIWorkspacesClient) Create(name, location, groupID, plan string) (*resourcecontrollerv2.ResourceInstance, *core.DetailedResponse, error) {
	resourceController, err := ibmpisession.CreateResourceControllerV2(f.session.Options.URL, f.session.Options.Authenticator)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Resource Controller client: %v", err)
	}
	planID := translatePlan(plan)
	if planID == "" {
		return nil, nil, fmt.Errorf("workspace creation error, incorrect plan value; either \"public\" or \"private\" is allowed")
	}
	params := resourceController.NewCreateResourceInstanceOptions(name, location, groupID, planID)
	workspace, response, err := resourceController.CreateResourceInstance(params)
	if err != nil {
		return nil, response, fmt.Errorf("error creating workspace: workspace %v response %v  err %v", workspace, response, err)
	}
	if response.StatusCode >= 400 {
		return nil, response, fmt.Errorf("error creating resource instance. Status code: %d", response.StatusCode)
	}
	return workspace, response, nil
}

// Create a workspace with parameters
func (f *IBMPIWorkspacesClient) CreateV2(name, location, groupID, plan string, parameters map[string]interface{}) (*resourcecontrollerv2.ResourceInstance, *core.DetailedResponse, error) {
	resourceController, err := ibmpisession.CreateResourceControllerV2(f.session.Options.URL, f.session.Options.Authenticator)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Resource Controller client: %v", err)
	}
	planID := translatePlan(plan)
	if planID == "" {
		return nil, nil, fmt.Errorf("workspace creation error, incorrect plan value; either \"public\" or \"private\" is allowed")
	}
	params := resourceController.NewCreateResourceInstanceOptions(name, location, groupID, planID)
	if len(parameters) > 0 {
		params.SetParameters(parameters)
	}
	workspace, response, err := resourceController.CreateResourceInstance(params)
	if err != nil {
		return nil, response, fmt.Errorf("error creating workspace: workspace %v response %v  err %v", workspace, response, err)
	}
	if response.StatusCode >= 400 {
		return nil, response, fmt.Errorf("error creating resource instance. Status code: %d", response.StatusCode)
	}
	return workspace, response, nil
}

// Delete a workspace
func (f *IBMPIWorkspacesClient) Delete(workspaceID string) (*core.DetailedResponse, error) {
	resourceController, err := ibmpisession.CreateResourceControllerV2(f.session.Options.URL, f.session.Options.Authenticator)
	if err != nil {
		return nil, fmt.Errorf("error creating Resource Controller client: %v", err)
	}
	params := resourceController.NewDeleteResourceInstanceOptions(workspaceID)
	response, err := resourceController.DeleteResourceInstance(params)
	if err != nil {
		return response, fmt.Errorf("error deleting workspace: response %v, err %v", response, err)
	}
	if response.StatusCode >= 400 {
		return response, fmt.Errorf("error deleting resource instance. Status code: %d", response.StatusCode)
	}
	return response, nil
}

// Get a resourceController
func (f *IBMPIWorkspacesClient) GetRC(rcWorkspaceID string) (*resourcecontrollerv2.ResourceInstance, *core.DetailedResponse, error) {
	resourceController, err := ibmpisession.CreateResourceControllerV2(f.session.Options.URL, f.session.Options.Authenticator)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Resource Controller client: %v", err)
	}
	params := resourceController.NewGetResourceInstanceOptions(rcWorkspaceID)
	workspace, response, err := resourceController.GetResourceInstance(params)
	if err != nil {
		return nil, response, fmt.Errorf("error creating workspace: workspace %v response %v  err %v", workspace, response, err)
	}
	if response.StatusCode >= 400 {
		return nil, response, fmt.Errorf("error getting resource instance. Status code: %d", response.StatusCode)
	}
	return workspace, response, nil
}
