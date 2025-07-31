package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_volume_onboarding"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIVolumeOnboardingClient
type IBMPIVolumeOnboardingClient struct {
	IBMPIClient
}

// NewIBMPIVolumeOnboardingClient
func NewIBMPIVolumeOnboardingClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIVolumeOnboardingClient {
	return &IBMPIVolumeOnboardingClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get the information of volume onboarding operation
func (f *IBMPIVolumeOnboardingClient) Get(id string) (*models.VolumeOnboarding, error) {
	params := p_cloud_volume_onboarding.NewPcloudVolumeOnboardingGetParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithVolumeOnboardingID(id)
	resp, err := f.session.Power.PCloudVolumeOnboarding.PcloudVolumeOnboardingGet(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetVolumeOnboardingOperationFailed, id, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Volume Onboarding for volume-onboarding ID:%s", id)
	}
	return resp.Payload, nil
}

// Get All volume onboardings for this cloud instance
func (f *IBMPIVolumeOnboardingClient) GetAll() (*models.VolumeOnboardings, error) {
	params := p_cloud_volume_onboarding.NewPcloudVolumeOnboardingGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudVolumeOnboarding.PcloudVolumeOnboardingGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetAllVolumeOnboardingsOperationFailed, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get All Volume Onboardings for the cloud instance %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}

// Onboard auxiliary volumes to target site
func (f *IBMPIVolumeOnboardingClient) CreateVolumeOnboarding(body *models.VolumeOnboardingCreate) (*models.VolumeOnboardingCreateResponse, error) {
	params := p_cloud_volume_onboarding.NewPcloudVolumeOnboardingPostParams().
		WithContext(f.ctx).WithTimeout(helpers.PICreateTimeOut).
		WithCloudInstanceID(f.cloudInstanceID).WithBody(body)
	resp, err := f.session.Power.PCloudVolumeOnboarding.PcloudVolumeOnboardingPost(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.CreateVolumeOnboardingsOperationFailed, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Create Volume Onboarding")
	}
	return resp.Payload, nil
}
