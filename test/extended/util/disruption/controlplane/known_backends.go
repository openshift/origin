package controlplane

import (
	"context"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"

	"k8s.io/client-go/rest"
)

func StartAllAPIMonitoring(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	if err := startKubeAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithNewConnections(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig); err != nil {
		return err
	}
	if err := startOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(ctx, m, clusterConfig); err != nil {
		return err
	}
	return nil
}

func startKubeAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithNewConnections(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startKubeAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func startOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	backendSampler, err := createOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m, nil)
}

func createKubeAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", backenddisruption.NewConnectionType)
}

func createKubeAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	return createAPIServerBackendSampler(clusterConfig, "cache-kube-api", "/api/v1/namespaces/default?resourceVersion=0", backenddisruption.NewConnectionType)
}

func createOpenShiftAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", backenddisruption.NewConnectionType)
}

func createOpenShiftAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "cache-openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", backenddisruption.NewConnectionType)
}

func createOAuthAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", backenddisruption.NewConnectionType)
}

func createOAuthAPIMonitoringWithNewConnectionsAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "cache-oauth-api", "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", backenddisruption.NewConnectionType)
}

func createKubeAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", backenddisruption.ReusedConnectionType)
}

func createKubeAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "cache-kube-api", "/api/v1/namespaces/default?resourceVersion=0", backenddisruption.ReusedConnectionType)
}

func createOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", backenddisruption.ReusedConnectionType)
}

func createOpenShiftAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "cache-openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams?resourceVersion=0", backenddisruption.ReusedConnectionType)
}

func createOAuthAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", backenddisruption.ReusedConnectionType)
}

func createOAuthAPIMonitoringWithConnectionReuseAgainstAPICache(clusterConfig *rest.Config) (*backenddisruption.BackendSampler, error) {
	// by setting resourceVersion="0" we instruct the server to get the data from the memory cache and avoid contacting with the etcd.
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "cache-oauth-api", "/apis/oauth.openshift.io/v1/oauthclients?resourceVersion=0", backenddisruption.ReusedConnectionType)
}

func createAPIServerBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType backenddisruption.BackendConnectionType) (*backenddisruption.BackendSampler, error) {
	// default gets auto-created, so this should always exist
	backendSampler, err := backenddisruption.NewAPIServerBackend(clusterConfig, disruptionBackendName, url, connectionType)
	if err != nil {
		return nil, err
	}

	return backendSampler, nil
}
