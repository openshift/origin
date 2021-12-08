package monitor

import (
	"context"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// this entire file should be a separate package with disruption_***, but we are entanged because the sampler lives in monitor
// and the things being started by the monitor are coupled into .Start.
// we also got stuck on writing the disruption backends.  We need a way to track which disruption checks we have started,
// so we can properly write out "zero"

var (
	LocatorKubeAPIServerNewConnection         = LocateDisruptionCheck("kube-api", NewConnectionType)
	LocatorKubeAPIServerReusedConnection      = LocateDisruptionCheck("kube-api", ReusedConnectionType)
	LocatorOpenshiftAPIServerNewConnection    = LocateDisruptionCheck("openshift-api", NewConnectionType)
	LocatorOpenshiftAPIServerReusedConnection = LocateDisruptionCheck("openshift-api", ReusedConnectionType)
	LocatorOAuthAPIServerNewConnection        = LocateDisruptionCheck("oauth-api", NewConnectionType)
	LocatorOAuthAPIServerReusedConnection     = LocateDisruptionCheck("oauth-api", ReusedConnectionType)
)

// BackendDisruptionLocatorsToName maps from the locator name used to track disruption to the name used to recognize it in
// the job aggregator.
var BackendDisruptionLocatorsToName = map[string]string{
	LocatorKubeAPIServerNewConnection:         "kube-api-new-connections",
	LocatorOpenshiftAPIServerNewConnection:    "openshift-api-new-connections",
	LocatorOAuthAPIServerNewConnection:        "oauth-api-new-connections",
	LocatorKubeAPIServerReusedConnection:      "kube-api-reused-connections",
	LocatorOpenshiftAPIServerReusedConnection: "openshift-api-reused-connections",
	LocatorOAuthAPIServerReusedConnection:     "oauth-api-reused-connections",
	LocateRouteForDisruptionCheck("openshift-authentication", "oauth-openshift", "ingress-to-oauth-server", NewConnectionType):    "ingress-to-oauth-server-new-connections",
	LocateRouteForDisruptionCheck("openshift-authentication", "oauth-openshift", "ingress-to-oauth-server", ReusedConnectionType): "ingress-to-oauth-server-used-connections",
	LocateRouteForDisruptionCheck("openshift-console", "console", "ingress-to-console", NewConnectionType):                        "ingress-to-console-new-connections",
	LocateRouteForDisruptionCheck("openshift-console", "console", "ingress-to-console", ReusedConnectionType):                     "ingress-to-console-used-connections",
	LocateRouteForDisruptionCheck("openshift-image-registry", "test-disruption", "image-registry", NewConnectionType):             "image-registry-new-connections",
	LocateRouteForDisruptionCheck("openshift-image-registry", "test-disruption", "image-registry", ReusedConnectionType):          "image-registry-reused-connections",
	LocateDisruptionCheck("service-loadbalancer-with-pdb", NewConnectionType):                                                     "service-load-balancer-with-pdb-new-connections",
	LocateDisruptionCheck("service-loadbalancer-with-pdb", ReusedConnectionType):                                                  "service-load-balancer-with-pdb-reused-connections",
}

func startKubeAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateKubeAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func startOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOpenShiftAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func startOAuthAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOAuthAPIMonitoringWithNewConnections(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func startKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateKubeAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func startOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func startOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config) error {
	backendSampler, err := CreateOAuthAPIMonitoringWithConnectionReuse(clusterConfig)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, m)
}

func CreateKubeAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*BackendSampler, error) {
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", NewConnectionType)
}

func CreateOpenShiftAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", NewConnectionType)
}

func CreateOAuthAPIMonitoringWithNewConnections(clusterConfig *rest.Config) (*BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", NewConnectionType)
}

func CreateKubeAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*BackendSampler, error) {
	// default gets auto-created, so this should always exist
	return createAPIServerBackendSampler(clusterConfig, "kube-api", "/api/v1/namespaces/default", ReusedConnectionType)
}

func CreateOpenShiftAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*BackendSampler, error) {
	// this request should never 404, but should be empty/small
	return createAPIServerBackendSampler(clusterConfig, "openshift-api", "/apis/image.openshift.io/v1/namespaces/default/imagestreams", ReusedConnectionType)
}

func CreateOAuthAPIMonitoringWithConnectionReuse(clusterConfig *rest.Config) (*BackendSampler, error) {
	// this should be relatively small and should not ever 404
	return createAPIServerBackendSampler(clusterConfig, "oauth-api", "/apis/oauth.openshift.io/v1/oauthclients", ReusedConnectionType)
}

func createAPIServerBackendSampler(clusterConfig *rest.Config, disruptionBackendName, url string, connectionType BackendConnectionType) (*BackendSampler, error) {
	kubeTransportConfig, err := clusterConfig.TransportConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(kubeTransportConfig)
	if err != nil {
		return nil, err
	}
	// default gets auto-created, so this should always exist
	backendSampler :=
		NewBackend(disruptionBackendName, url, connectionType).
			WithHost(clusterConfig.Host).
			WithTLSConfig(tlsConfig).
			WithBearerTokenAuth(kubeTransportConfig.BearerToken, kubeTransportConfig.BearerTokenFile)

	return backendSampler, nil
}
