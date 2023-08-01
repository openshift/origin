package frontends

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	oauthRouteNamespace = "openshift-authentication"
	oauthRouteName      = "oauth-openshift"
)

func StartAllIngressMonitoring(ctx context.Context, m monitorapi.Recorder, clusterConfig *rest.Config, _ backend.LoadBalancerType) error {
	// Ingress monitoring checks for oauth and console routes to monitor healthz endpoints. Check availability
	// before setting up any monitors.
	routeAvailable, err := isRouteAvailable(ctx, clusterConfig, oauthRouteNamespace, oauthRouteName)
	if err != nil {
		return err
	}
	if routeAvailable {
		if err := createOAuthRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
		if err := createOAuthRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
	}

	configAvailable, err := exutil.DoesApiResourceExist(clusterConfig, "clusterversions", "config.openshift.io")
	if err != nil {
		return err
	}
	if configAvailable {
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
		if !knowsCapability(clusterVersion, "Console") ||
			hasCapability(clusterVersion, "Console") {
			if err := CreateConsoleRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
				return err
			}
			if err := createConsoleRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func hasCapability(clusterVersion *configv1.ClusterVersion, desiredCapability string) bool {
	for _, ec := range clusterVersion.Status.Capabilities.EnabledCapabilities {
		if string(ec) == desiredCapability {
			return true
		}
	}
	return false
}
func knowsCapability(clusterVersion *configv1.ClusterVersion, desiredCapability string) bool {
	for _, ec := range clusterVersion.Status.Capabilities.KnownCapabilities {
		if string(ec) == desiredCapability {
			return true
		}
	}
	return false
}

func isRouteAvailable(ctx context.Context, config *rest.Config, namespace, name string) (bool, error) {
	routeClient, err := routeclient.NewForConfig(config)
	if err != nil {
		return false, err
	}
	_, err = routeClient.RouteV1().Routes(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func createOAuthRouteAvailableWithNewConnections() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		oauthRouteNamespace,
		oauthRouteName,
		"ingress-to-oauth-server",
		"/healthz",
		monitorapi.NewConnectionType).
		WithExpectedBody("ok")
}

func createOAuthRouteAvailableWithConnectionReuse() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		oauthRouteNamespace,
		oauthRouteName,
		"ingress-to-oauth-server",
		"/healthz",
		monitorapi.ReusedConnectionType).
		WithExpectedBody("ok")
}

func CreateConsoleRouteAvailableWithNewConnections() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewRouteBackend(
		restConfig,
		"openshift-console",
		"console",
		"ingress-to-console",
		"/healthz",
		monitorapi.NewConnectionType).
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
		monitorapi.ReusedConnectionType).
		WithExpectedBodyRegex(`(Red Hat OpenShift|OKD)`)
}
