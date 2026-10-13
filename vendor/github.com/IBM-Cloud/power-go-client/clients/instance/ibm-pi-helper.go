package instance

import (
	"context"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
)

// Helper methods that will be used by the client classes

// IBMPIHelperClient
type IBMPIClient struct {
	session         *ibmpisession.IBMPISession
	cloudInstanceID string
	ctx             context.Context
}

// NewIBMPIClient
func NewIBMPIClient(ctx context.Context, sess *ibmpisession.IBMPISession, cloudInstanceID string) *IBMPIClient {
	return &IBMPIClient{
		session:         sess,
		cloudInstanceID: cloudInstanceID,
		ctx:             ctx,
	}
}
