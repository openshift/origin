package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/transport"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/origin/test/extended/util"
	promHelpers "github.com/openshift/origin/test/extended/util/prometheus"
)

// NewE2EPrometheusRouterClient returns a Prometheus HTTP API client configured to
// use the Prometheus route host, a bearer token, and no certificate verification.
func NewE2EPrometheusRouterClient(oc *util.CLI) (prometheusv1.API, error) {
	kubeClient := oc.AdminKubeClient()
	routeClient := oc.AdminRouteClient()

	// wait for prometheus service to exist
	err := wait.PollImmediate(time.Minute, time.Second, func() (bool, error) {
		_, err := kubeClient.CoreV1().Services("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
		return err == nil, nil
	})
	if err != nil {
		return nil, err
	}

	// wait for the prometheus route to exist
	var route *routev1.Route
	err = wait.PollImmediate(time.Minute, time.Second, func() (bool, error) {
		route, err = routeClient.RouteV1().Routes("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
		return err == nil, nil
	})
	if err != nil {
		return nil, err
	}

	token := promHelpers.GetPrometheusSABearerToken(oc)

	// prometheus API client, configured for route host and bearer token auth, and no cert verification
	client, err := api.NewClient(api.Config{
		Address: "https://" + route.Status.Ingress[0].Host,
		RoundTripper: transport.NewBearerAuthRoundTripper(
			token,
			&http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			},
		),
	})
	if err != nil {
		return nil, err
	}

	// return prometheus API
	return prometheusv1.NewAPI(client), nil
}
