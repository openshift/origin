package metrics

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/transport"

	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
)

// NewPrometheusClient returns Prometheus API or error
// Note: with thanos-querier you must pass an entire Alert as a query. Partial queries return an error, so have to pass the entire alert.
// Example query for an Alert:
// `ALERTS{alertname="PodDisruptionBudgetAtLimit",alertstate="pending",namespace="pdbnamespace",poddisruptionbudget="pdbname",prometheus="openshift-monitoring/k8s",service="kube-state-metrics",severity="warning"}==1`
// Example query:
// `scheduler_scheduling_duration_seconds_sum`
func NewPrometheusClient(ctx context.Context, kclient *kubernetes.Clientset, rc *routeclient.Clientset) (prometheusv1.API, error) {
	_, err := kclient.CoreV1().Services("openshift-monitoring").Get(ctx, "prometheus-k8s", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	route, err := rc.RouteV1().Routes("openshift-monitoring").Get(ctx, "thanos-querier", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	host := route.Status.Ingress[0].Host
	var bearerToken string
	secrets, err := kclient.CoreV1().Secrets("openshift-monitoring").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list secrets in openshift-monitoring namespace")
	}
	for _, s := range secrets.Items {
		if s.Type != corev1.SecretTypeServiceAccountToken ||
			!strings.HasPrefix(s.Name, "prometheus-k8s") {
			continue
		}
		bearerToken = string(s.Data[corev1.ServiceAccountTokenKey])
		break
	}
	if len(bearerToken) == 0 {
		return nil, fmt.Errorf("prometheus service account not found")
	}

	return createClient(ctx, kclient, host, bearerToken)
}

func createClient(ctx context.Context, kclient *kubernetes.Clientset, host, bearerToken string) (prometheusv1.API, error) {
	// retrieve router CA
	routerCAConfigMap, err := kclient.CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	bundlePEM := []byte(routerCAConfigMap.Data["ca-bundle.crt"])

	// make a client connection configured with the provided bundle.
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(bundlePEM)

	// prometheus API client, configured for route host and bearer token auth
	client, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: "https://" + host,
		RoundTripper: transport.NewBearerAuthRoundTripper(
			bearerToken,
			&http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
				TLSClientConfig: &tls.Config{
					RootCAs:    roots,
					ServerName: host,
				},
			},
		),
	})
	if err != nil {
		return nil, err
	}
	return prometheusv1.NewAPI(client), nil
}
