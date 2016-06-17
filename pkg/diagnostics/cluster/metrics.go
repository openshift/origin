package cluster

// The purpose of this diagnostic is to test whether the API proxy is
// properly configured so that the HPA can reach metrics

import (
	"errors"
	"fmt"

	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/diagnostics/types"
)

// MetricsApiProxy is a Diagnostic for diagnosing the API proxy and HPA/metrics.
type MetricsApiProxy struct {
	KubeClient *kclient.Client
}

const (
	MetricsApiProxyName     = "MetricsApiProxy"
	MetricsApiProxyProject  = "openshift-infra"
	MetricsApiProxyService  = "heapster"
	errMsgNoHeapsterService = `The %[1]s service does not exist in the %[2]s project at this time,
so it is not available for the Horizontal Pod Autoscaler to use as a source of metrics.`
	errMsgNoHeapsterEndpoints = `The %[1]s service exists in the %[2]s project but does not have any endpoints at this time,
so it is not available for the Horizontal Pod Autoscaler to use as a source of metrics.`
	errMsgApiProxyAccess = `Unable to access the metrics API Proxy endpoint %[1]s:
(%[2]T) %[2]v
The Horizontal Pod Autoscaler is not able to retrieve metrics to drive scaling.`
)

func (d *MetricsApiProxy) Name() string {
	return MetricsApiProxyName
}

func (d *MetricsApiProxy) Description() string {
	return "Check the integrated heapster metrics can be reached via the API proxy"
}

func (d *MetricsApiProxy) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	}
	// see if there's even a service to reach - if not, they probably haven't deployed
	// metrics and don't need to get errors about it; skip the diagnostic
	if _, err := d.KubeClient.Services(MetricsApiProxyProject).Get(MetricsApiProxyService); kapierrors.IsNotFound(err) {
		return false, fmt.Errorf(errMsgNoHeapsterService, MetricsApiProxyService, MetricsApiProxyProject)
	} else if err != nil {
		return false, fmt.Errorf("Unexpected error while retrieving %[1]s service: (%[2]T) %[2]v", MetricsApiProxyService, err)
	}
	return true, nil
}

func (d *MetricsApiProxy) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(MetricsApiProxyName)

	// see if it has any active endpoints
	if endpoints, err := d.KubeClient.Endpoints(MetricsApiProxyProject).Get(MetricsApiProxyService); err != nil {
		r.Error("DClu4001", err, fmt.Sprintf("Unexpected error while retrieving %[1]s service endpoints: (%[2]T) %[2]v", MetricsApiProxyService, err))
		return r
	} else {
		active := false
		if endpoints.Subsets != nil {
			for _, endpoint := range endpoints.Subsets {
				if len(endpoint.Addresses) > 0 {
					active = true
					break
				}
			}
		}
		if !active {
			r.Error("DClu4002", nil, fmt.Sprintf(errMsgNoHeapsterEndpoints, MetricsApiProxyService, MetricsApiProxyProject))
			return r
		}
	}

	// the service should respond; see if we can reach it via API proxy
	uri := fmt.Sprintf("/api/v1/proxy/namespaces/%[1]s/services/https:%[2]s:/api/v1/model/metrics", MetricsApiProxyProject, MetricsApiProxyService)
	// note in above, project and service name are already URL-safe
	result := d.KubeClient.Get().RequestURI(uri).Do()
	if err := result.Error(); err != nil {
		r.Error("DClu4003", err, fmt.Sprintf(errMsgApiProxyAccess, uri, err))
	}
	return r
}
