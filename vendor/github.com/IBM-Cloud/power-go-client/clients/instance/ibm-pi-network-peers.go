package instance

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/network_peers"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

// IBMPINetworkPeerClient
type IBMPINetworkPeerClient struct {
	IBMPIClient
}

// NewIBMPINetworkPeerClient
func NewIBMPINetworkPeerClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPINetworkPeerClient {
	return &IBMPINetworkPeerClient{
		*NewIBMPIClient(ctx, sess, cloudInstanceID),
	}
}

// Get network peers
func (f *IBMPINetworkPeerClient) GetNetworkPeers() (*models.NetworkPeers, error) {
	if !f.session.IsOnPrem() {
		return nil, fmt.Errorf(helpers.NotOffPremSupported)
	}
	params := network_peers.NewV1NetworkPeersListParams().WithContext(f.ctx).
		WithTimeout(helpers.PIGetTimeOut)
	resp, err := f.session.Power.NetworkPeers.V1NetworkPeersList(params, f.session.AuthInfo(f.cloudInstanceID))
	if err != nil {
		return nil, ibmpisession.SDKFailWithAPIError(err, fmt.Errorf("failed to Get Network Peers for cloud instance %s with error %w", f.cloudInstanceID, err))
	}
	if resp == nil || resp.Payload == nil {
		return nil, fmt.Errorf("failed to Get Network Peers for cloud instance %s", f.cloudInstanceID)
	}
	return resp.Payload, nil
}
