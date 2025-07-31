package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/errors"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_storage_tiers"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPIStorageTierClient
type IBMPIStorageTierClient struct {
	IBMPIClient
}

// NewIBMPIStorageTierClient
func NewIBMPIStorageTierClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIStorageTierClient {
	return &IBMPIStorageTierClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Gets all the storage tiers associated to the cloud instance.
func (f *IBMPIStorageTierClient) GetAll() (models.RegionStorageTiers, error) {
	params := p_cloud_storage_tiers.NewPcloudCloudinstancesStoragetiersGetallParams().
		WithContext(f.ctx).WithTimeout(helpers.PIGetTimeOut).
		WithCloudInstanceID(f.cloudInstanceID)
	resp, err := f.session.Power.PCloudStorageTiers.PcloudCloudinstancesStoragetiersGetall(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf(errors.GetAllStorageTiersOperationFailed, f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to get all storage tiers")
	}
	return resp.Payload, nil
}
