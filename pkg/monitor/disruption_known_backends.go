package monitor

import (
	"context"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"

	"k8s.io/client-go/rest"
)

// this entire file should be a separate package with disruption_***, but we are entanged because the sampler lives in monitor
// and the things being started by the monitor are coupled into .Start.
// we also got stuck on writing the disruption backends.  We need a way to track which disruption checks we have started,
// so we can properly write out "zero"

func startKubeAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateKubeAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOpenShiftAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOAuthAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateKubeAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOAuthAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func CreateKubeAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", backenddisruption.NewConnectionType)
}

func CreateOpenShiftAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", backenddisruption.NewConnectionType)
}

func CreateOAuthAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", backenddisruption.NewConnectionType)
}

func CreateKubeAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", backenddisruption.ReusedConnectionType)
}

func CreateOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", backenddisruption.ReusedConnectionType)
}

func CreateOAuthAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", backenddisruption.ReusedConnectionType)
}

func createAPIServerBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType backenddisruption.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}

	return backendSampler, nil
}
