package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_s_a_p"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPISAPInstanceClient
type IBMPISAPInstanceClient struct {
	IBMPIClient
}

// NewIBMPISAPInstanceClient
func NewIBMPISAPInstanceClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPISAPInstanceClient {
	return &IBMPISAPInstanceClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Create a SAP Instance
func (f *IBMPISAPInstanceClient) Create(body *models.SAPCreate) (*models.PVMInstanceList, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_s_a_p.NewPcloudSapPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	postok, postcreated, postAccepted, err := f.session.Power.PCloudsap.PcloudSapPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Create SAP Instance: %w", err))
	}
	if postok != nil && len(postok.Payload) > 0 {
		return &postok.Payload, nil
	}
	if postcreated != nil && len(postcreated.Payload) > 0 {
		return &postcreated.Payload, nil
	}
	if postAccepted != nil && len(postAccepted.Payload) > 0 {
		return &postAccepted.Payload, nil
	}
	return nil, fmt.Errorf("failed to Create SAP Instance")
}

// Get a SAP Profile
func (f *IBMPISAPInstanceClient) GetSAPProfile(id string) (*models.SAPProfile, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_s_a_p.NewPcloudSapGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithSapProfileID(id)
	resp, err := f.session.Power.PCloudsap.PcloudSapGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get sap profile %s : %w", id, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get sap profile %s", id)
	}
	return resp.Payload, nil
}

// Get All SAP Profiles
func (f *IBMPISAPInstanceClient) GetAllSAPProfiles(cloudInstanceID string) (*models.SAPProfiles, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}
	params := p_cloud_s_a_p.NewPcloudSapGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudsap.PcloudSapGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get all sap profiles for power instance %s: %w", cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get all sap profiles for power instance %s", cloudInstanceID)
	}
	return resp.Payload, nil
}

// Get All SAP Profiles with filters
func (f *IBMPISAPInstanceClient) GetAllSAPProfilesWithFilters(cloudInstanceID string, filterMap map[string]string) (*models.SAPProfiles, error) {
	if f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOnPremSupported)
	}

	familyFilter := filterMap[helpers.PISAPProfileFamilyFilterMapKey]
	prefixFilter := filterMap[helpers.PISAPProfilePrefixFilterMapKey]

	params := p_cloud_s_a_p.NewPcloudSapGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithProfilePrefix(&prefixFilter).
		WithProfileFamily(&familyFilter)

	resp, err := f.session.Power.PCloudsap.PcloudSapGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to get all sap profiles for power instance %s: %w", cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get all sap profiles for power instance %s", cloudInstanceID)
	}
	return resp.Payload, nil
}
