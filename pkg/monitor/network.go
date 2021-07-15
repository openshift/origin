package monitor

import (
	"context"
	"time"

	"k8s.io/client-go/rest"
)

func StartKubeNetworkMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartNetworkMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorKubeAPIServerNewConnection, "/api/v1/namespaces/default")
}

func StartOpenShiftNetworkMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartNetworkMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorOpenshiftAPIServerNewConnection, "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthNetworkMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartNetworkMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorOAuthAPIServerNewConnection, "/apis/oauth.openshift.io/v1/oauthclients")
}

func StartKubeNetworkMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartNetworkMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorKubeAPIServerReusedConnection, "/api/v1/namespaces/default")
}

func StartOpenShiftNetworkMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartNetworkMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorOpenshiftAPIServerReusedConnection, "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthNetworkMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartNetworkMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorOAuthAPIServerReusedConnection, "/apis/oauth.openshift.io/v1/oauthclients")
}
