package frontends

import (
	"context"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"k8s.io/client-go/rest"
)

func StartAllIngressMonitoring(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	if err := createOAuthRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}
	if err := createOAuthRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}
	if err := createConsoleRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}
	if err := createConsoleRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}
	return nil
}

func createOAuthRouteAvailableWithNewConnections() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		"openshift-authentication",
		"oauth-openshift",
		"ingress-to-oauth-server",
		"/healthz",
		backenddisruption.NewConnectionType).
		WithExpectedBody("ok")
}

func createOAuthRouteAvailableWithConnectionReuse() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		"openshift-authentication",
		"oauth-openshift",
		"ingress-to-oauth-server",
		"/healthz",
		backenddisruption.ReusedConnectionType).
		WithExpectedBody("ok")
}

func createConsoleRouteAvailableWithNewConnections() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		"openshift-console",
		"console",
		"ingress-to-console",
		"/healthz",
		backenddisruption.NewConnectionType).
		WithExpectedBodyRegex(`(Red Hat OpenShift Container Platform|OKD)`)
}

func createConsoleRouteAvailableWithConnectionReuse() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		"openshift-console",
		"console",
		"ingress-to-console",
		"/healthz",
		backenddisruption.ReusedConnectionType).
		WithExpectedBodyRegex(`(Red Hat OpenShift Container Platform|OKD)`)
}
