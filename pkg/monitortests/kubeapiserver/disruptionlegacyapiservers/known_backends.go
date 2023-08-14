package disruptionlegacyapiservers

import (
	"fmt"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/client-go/rest"
)

func createAPIServerBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType monitorapi.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}
	backendSampler = backendSampler.WithUserAgent(fmt.Sprintf("openshift-external-backend-sampler-%s-%s", connectionType, disruptionBackendName))

	return backendSampler, nil
}
