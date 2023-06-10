package clientresterroranalyzer

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

var scrapePeriod = 30 * time.Second

func fetchEventIntervalsForRestClientError(ctx context.Context, config *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, "openshift-monitoring", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return []monitorapi.Interval{}, nil
	}

	kubeSvc, err := kubeClient.CoreV1().Services(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster IP of kubernetes.default.svc - %v", err)
	}

	client, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	framework.Logf("[client-rest-error-serializer] collecting rest metrics")
	gatherer := &gatherer{start: startTime, client: client, serviceNetworkIP: kubeSvc.Spec.ClusterIP}

	// TODO: build more robust range query - run multiple range
	//  queries with smaller time range ex: {from - from+30m}
	result, err := gatherer.query(ctx)
	if err != nil {
		return nil, fmt.Errorf("metric query for rest client failed: %v", err)
	}

	return gatherer.parse(result)
}

type gatherer struct {
	start            time.Time
	client           prometheusv1.API
	serviceNetworkIP string
}

func (g *gatherer) query(ctx context.Context) (prometheustypes.Value, error) {
	// TODO: should we include 5xx? "429|5..|<error>"
	query := `rest_client_requests_total{code =~ "429|5..|<error>"}`
	result, warnings, err := g.client.QueryRange(ctx, query, prometheusv1.Range{
		Start: g.start,
		End:   time.Now(),
		// data is scraped every 30s in OpenShift
		Step: scrapePeriod,
	})
	if err != nil {
		framework.Logf("[client-rest-error-serializer]: prometheus client returned error: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		framework.Logf("[client-rest-error-serializer]: #### warnings \\n\\t%v\\n\", strings.Join(warningsForQuery, \"\\n\\t\"", warnings)
	}

	return result, nil
}

func (g *gatherer) batchIntervals(intervals []monitorapi.Interval, locator string, current time.Time) ([]monitorapi.Interval, bool) {
	foundIndex := -1
	// Find latest locator in existing intervals matching the locat
	for index, interval := range intervals {
		if interval.Locator == locator && interval.From == current {
			foundIndex = index
			break
		}
	}
	// No match found
	if foundIndex == -1 {
		return intervals, true
	}
	// Extend existing interval
	intervals[foundIndex].To = intervals[foundIndex].From.Add(scrapePeriod)
	framework.Logf("[client-rest-error-serializer] updated existing interval %+v", intervals[foundIndex])
	return intervals, foundIndex == -1
}

func (g *gatherer) parse(result prometheustypes.Value) ([]monitorapi.Interval, error) {
	intervals := []monitorapi.Interval{}
	if result.Type() != prometheustypes.ValMatrix {
		return nil, fmt.Errorf("expected a Matrix")
	}

	matrix := result.(prometheustypes.Matrix)
	framework.Logf("[client-rest-error-serializer]: found %v series", len(matrix))
	for _, series := range matrix {
		if len(series.Values) <= 1 {
			continue
		}
		previous := series.Values[0].Value
		for _, current := range series.Values[1:] {
			if !previous.Equal(current.Value) {
				// we have a change in the count
				// TODO: counter can reset to zero, we need to handle it,
				//  if we want to display the increment.
				source := source(g.serviceNetworkIP, series.Metric)
				component := component(series.Metric)
				namespace := string(series.Metric["namespace"])
				locator := fmt.Sprintf("client/APIError source/%s node/%s namespace/%s component/%s", source, instance(series.Metric), namespace, component)

				if newIntervals, needsNew := g.batchIntervals(intervals, locator, current.Timestamp.Time()); !needsNew {
					intervals = newIntervals
					continue
				}

				// TODO: the tool tip in e2e timeline does not display
				//  code=<error>, maybe use html.EscapeString()?
				if series.Metric["code"] == "<error>" {
					series.Metric["code"] = "error"
				}
				interval := monitorapi.Interval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locator,
						Message: fmt.Sprintf("client observed an API error - previous=%s current=%s %s",
							previous.String(), current.Value.String(), series.Metric.String()),
					},
					From: current.Timestamp.Time(),
					// TODO: find how long did the requests took using data from rest_client_request_duration_seconds_sum?
					To: current.Timestamp.Time().Add(time.Second),
				}
				framework.Logf("[client-rest-error-serializer] adding new interval %+v", interval)
				intervals = append(intervals, interval)
			}
			previous = current.Value
		}
	}

	return intervals, nil
}

func instance(metric prometheustypes.Metric) string {
	// instance="10.128.0.16:8441"
	hostPort := string(metric["instance"])
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

// the endpoint used to reach the apiserver
func source(serviceNetwork string, metric prometheustypes.Metric) string {
	host := string(metric["host"])
	switch {
	case strings.HasPrefix(host, "api-int."):
		return "internal-lb"
	case strings.HasPrefix(host, "api."):
		return "external-lb"
	case strings.HasPrefix(host, "[::1]:6443"):
		return "localhost"
	case strings.HasPrefix(host, serviceNetwork):
		return "service-network"
	}

	return host
}

func component(metric prometheustypes.Metric) string {
	service := string(metric["service"])
	// TODO: this will need more tweaking, to make sure
	//  all component names show up correctly.
	switch {
	case service == "kubernetes":
		return "kube-apiserver"
	case service == "api":
		return "openshift-apiserver"
	case service == "metrics":
		// TODO: many components use 'metrics' as the default service name,
		//  so we use the namespace as the component
		return string(metric["namespace"])
	default:
		return service
	}
}
