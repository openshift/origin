package frontends

import (
	"context"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/test/extended/util/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func StartAllIngressMonitoring(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	if err := createOAuthRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}
	if err := createOAuthRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
		return err
	}

	// Some jobs explicitly disable the console and other features. Check if it's disabled and if so,
	// do not run a disruption monitoring backend for it.
	configClient, err := configclient.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		e2e.Failf("Failed to get cluster version: %v", err)
	}
	// If the cluster does not know about the Console capability, it likely predates 4.12 and we can assume
	// it has it by default. This is to catch possible future scenarios where we upgrade 4.11 no cap to 4.12 no cap.
	if !cluster.KnowsCapability(clusterVersion, "Console") ||
		cluster.HasCapability(clusterVersion, "Console") {
		if err := createConsoleRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
		if err := createConsoleRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
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
		WithExpectedBodyRegex(`(Red Hat OpenShift|OKD)`)
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
		WithExpectedBodyRegex(`(Red Hat OpenShift|OKD)`)
}
