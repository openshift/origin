package metricsendpointdown

import (
	"context"
	"math"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/prometheus"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func buildIntervalsForMetricsEndpointsDown(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "buildIntervalsForMetricsEndpointsDown")
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, "openshift-monitoring", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return []monitorapi.Interval{}, nil
	}

	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	intervals, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient)
	if err != nil {
		return intervals, err
	}

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  2 * time.Second,
	}

	// query for when prom samples the kubelet /metrics and /metrics/cadvisor endpoints down
	outages, warningsForQuery, err := prometheusClient.QueryRange(ctx, `max by (node, instance, metrics_path, namespace, service) (up{service="kubelet"}) == 0`, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		if len(warningsForQuery) > 0 {
			for _, w := range warningsForQuery {
				logger.Warnf("target down prom query warning: %s", w)
			}
		}
	}

	firingAlerts, err := createIntervalsFromPrometheusSamples(logger, outages)
	if err != nil {
		return nil, err
	}

	ret := []monitorapi.Interval{}
	ret = append(ret, firingAlerts...)

	return ret, nil
}

func createIntervalsFromPrometheusSamples(logger logrus.FieldLogger, promVal prometheustypes.Value) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			logger.Infof("temp: got prom sampleStream: +%v", promSampleStream)

			lb := monitorapi.NewLocator().AlertFromPromSampleStream(promSampleStream)

			msg := monitorapi.NewMessage().HumanMessage(promSampleStream.Metric.String())
			intervalTmpl :=
				monitorapi.NewInterval(monitorapi.SourceMetricsEndpointDown, monitorapi.Warning).
					Locator(lb).
					Message(msg).Display()

			// Logic stolen from alertanalyzer/alerts.go
			var outageStart *time.Time
			var outageLast *time.Time
			for _, currValue := range promSampleStream.Values {
				logger.Infof("temp: got prom value: +%v", currValue)
				currTime := currValue.Timestamp.Time()
				if outageStart == nil {
					outageStart = &currTime
				}
				if outageLast == nil {
					outageLast = &currTime
				}
				// if it has been less than five seconds since we saw this, consider it the same interval and check
				// the next time.
				if math.Abs(currTime.Sub(*outageLast).Seconds()) < (5 * time.Second).Seconds() {
					outageLast = &currTime
					continue
				}

				// if it has been more than five seconds, consider this the start of a new occurrence and add the interval
				ret = append(ret, intervalTmpl.Build(*outageStart, *outageLast))

				// now reset the tracking
				outageStart = &currTime
				outageLast = nil
			}

			// now add the one for the last start time.  If we do not have a last time, it means we saw the start, but not
			// the end.  We don't know when this outage ended, but our threshold time from above is five seconds so we will
			// simply assign that here as "better than nothing"
			if outageLast == nil {
				t := outageStart.Add(5 * time.Second)
				outageLast = &t
			}
			ret = append(ret, intervalTmpl.Build(*outageStart, *outageLast))
		}

	default:
		logger.WithField("type", promVal.Type()).Warning("unhandled prometheus type received")
	}

	return ret, nil
}
