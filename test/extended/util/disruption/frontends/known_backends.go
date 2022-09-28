package frontends

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"sync"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/rest"
)

func StartAllIngressMonitoring(ctx context.Context, m monitor.Recorder, clusterConfig *rest.Config) error {
	configClient, err := configclient.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	topology, err := exutil.GetControlPlaneTopologyFromConfigClient(configClient)
	if err != nil {
		return err
	}

	// In external control plane topology mode (aka HyperShift) the OAuth server lives on the control plane side.
	// In this case, we watch the oauth endpoint by fetching it from the Kube APIServer .well-known URL
	if *topology == configv1.ExternalTopologyMode {
		if err := createOAuthWellKnownHostAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return nil
		}
		if err := createOAuthWellKnownHostAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
	} else {
		if err := createOAuthRouteAvailableWithNewConnections().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
		if err := createOAuthRouteAvailableWithConnectionReuse().StartEndpointMonitoring(ctx, m, nil); err != nil {
			return err
		}
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

func createOAuthWellKnownHostAvailableWithNewConnections() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewBackend(
		wellKnownOAuthHostGetter(restConfig),
		"openshift-authentication",
		"/healthz",
		backenddisruption.NewConnectionType).
		WithExpectedBody("ok")
}

func createOAuthWellKnownHostAvailableWithConnectionReuse() *backenddisruption.BackendSampler {
	restConfig, err := monitor.GetMonitorRESTConfig()
	utilruntime.Must(err)
	return backenddisruption.NewBackend(
		wellKnownOAuthHostGetter(restConfig),
		"openshift-authentication",
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

type oauthHostGetter struct {
	restConfig *rest.Config
	initialize sync.Once
	host       string
	hostErr    error
}

func (g *oauthHostGetter) GetHost() (string, error) {
	g.initialize.Do(func() {
		g.hostErr = func() error {
			client, err := rest.HTTPClientFor(g.restConfig)
			if err != nil {
				return err
			}
			u, err := url.Parse(g.restConfig.Host)
			if err != nil {
				return err
			}
			u.Path = path.Join(u.Path, ".well-known", "oauth-authorization-server")
			resp, err := client.Get(u.String())
			if err != nil {
				return err
			}
			respBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			oauthInfo := struct {
				Issuer string `json:"issuer"`
			}{}
			if err := json.Unmarshal(respBytes, &oauthInfo); err != nil {
				return err
			}
			g.host = oauthInfo.Issuer
			return nil
		}()
	})
	if g.hostErr != nil {
		return "", g.hostErr
	}
	if len(g.host) == 0 {
		return "", fmt.Errorf("missing URL")
	}
	return g.host, nil
}

func wellKnownOAuthHostGetter(cfg *rest.Config) *oauthHostGetter {
	return &oauthHostGetter{
		restConfig: cfg,
	}
}
