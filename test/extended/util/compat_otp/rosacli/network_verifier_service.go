package rosacli

import (
	"bytes"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

type NetworkVerifierService interface {
	ResourcesCleaner
	CreateNetworkVerifierWithCluster(clusterID string, flags ...string) (bytes.Buffer, error)
	CreateNetworkVerifierWithSubnets(flags ...string) (bytes.Buffer, error)
	GetNetworkVerifierStatus(flags ...string) (bytes.Buffer, error)
}

type networkVerifierService struct {
	ResourcesService

	nv map[string]string
}

func NewNetworkVerifierService(client *Client) NetworkVerifierService {
	return &networkVerifierService{
		ResourcesService: ResourcesService{
			client: client,
		},
		nv: make(map[string]string),
	}
}

func (nv *networkVerifierService) CreateNetworkVerifierWithCluster(clusterID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	createNetworkVerifier := nv.client.Runner.
		Cmd("verify", "network").
		CmdFlags(combflags...)

	return createNetworkVerifier.Run()
}

func (nv *networkVerifierService) CreateNetworkVerifierWithSubnets(flags ...string) (bytes.Buffer, error) {
	createNetworkVerifier := nv.client.Runner.
		Cmd("verify", "network").
		CmdFlags(flags...)

	return createNetworkVerifier.Run()
}

func (nv *networkVerifierService) GetNetworkVerifierStatus(flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"--watch", "--status-only"}, flags...)
	getNetworkVerifierStatus := nv.client.Runner.
		Cmd("verify", "network").
		CmdFlags(combflags...)

	return getNetworkVerifierStatus.Run()
}

func (nv *networkVerifierService) CleanResources(clusterID string) (errors []error) {
	logger.Debugf("Nothing to clean in NetworkVerifierService Service")
	return
}
