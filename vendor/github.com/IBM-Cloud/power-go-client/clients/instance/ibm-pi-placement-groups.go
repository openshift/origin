package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_placement_groups"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIPlacementGroupClient
type IBMPIPlacementGroupClient struct {
	IBMPIClient
}

// NewIBMPIPlacementGroupClient
func NewIBMPIPlacementGroupClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIPlacementGroupClient {
	return &IBMPIPlacementGroupClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get a PI Placement Group
func (f *IBMPIPlacementGroupClient) Get(id string) (*models.PlacementGroup, error) {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPlacementGroupID(id)
	resp, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetPlacementGroupOperationFailed, id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Placement Group %s", id)
	}
	return resp.Payload, nil
}

// Get All Placement Groups
func (f *IBMPIPlacementGroupClient) GetAll() (*models.PlacementGroups, error) {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get All Placement Groups: %w", err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get all Placement Groups")
	}
	return resp.Payload, nil
}

// Create a Placement Group
func (f *IBMPIPlacementGroupClient) Create(body *models.PlacementGroupCreate) (*models.PlacementGroup, error) {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	postok, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreatePlacementGroupOperationFailed, f.cloudInstanceID, err))
	}
	if postok == nil || postok.Payload == nil {
		return nil, fmt.Errorf("failed to Create Placement Group")
	}
	return postok.Payload, nil
}

// Delete a Placement Group
func (f *IBMPIPlacementGroupClient) Delete(id string) error {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPlacementGroupID(id)
	_, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return fmt.Errorf(errors.DeletePlacementGroupOperationFailed, id, err)
	}
	return nil
}

// Add an Instance to a Placement Group
func (f *IBMPIPlacementGroupClient) AddMember(id string, body *models.PlacementGroupServer) (*models.PlacementGroup, error) {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsMembersPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPlacementGroupID(id).
		WithBody(body)
	postok, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsMembersPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.AddMemberPlacementGroupOperationFailed, *body.ID, id, err))
	}
	if postok == nil || postok.Payload == nil {
		return nil, fmt.Errorf("failed to Add Member for instance %s and placement group %s", *body.ID, id)
	}
	return postok.Payload, nil
}

// Remove an Instance to a Placement Group
func (f *IBMPIPlacementGroupClient) DeleteMember(id string, body *models.PlacementGroupServer) (*models.PlacementGroup, error) {
	params := p_cloud_placement_groups.NewPcloudPlacementgroupsMembersDeleteParams().
		WithContext(f.ctx).WithTimeout(helpers.PIDeleteTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithPlacementGroupID(id).
		WithBody(body)
	delok, err := f.session.Power.PCloudPlacementGroups.PcloudPlacementgroupsMembersDelete(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.DeleteMemberPlacementGroupOperationFailed, *body.ID, id, err))
	}
	if delok == nil || delok.Payload == nil {
		return nil, fmt.Errorf("failed to Delete Member for instance %s and placement group %s", *body.ID, id)
	}
	return delok.Payload, nil
}
