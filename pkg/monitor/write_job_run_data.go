package monitor

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/test/extended/testdata"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// WriteRunDataToArtifactsDir attempts to write useful run data to the specified directory.
func WriteRunDataToArtifactsDir(ctx context.Context,
	artifactDir string, monitor *Monitor, events monitorapi.Intervals, timeSuffix string,
	restConfig *rest.Config, startTime time.Time) error {
	errors := []error{}
	if err := monitorserialization.EventsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), events); err != nil {
		errors = append(errors, err)
	} else {
	}
	if err := monitorserialization.EventsIntervalsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals%s.json", timeSuffix)), events); err != nil {
		errors = append(errors, err)
	}
	if eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(events); err == nil {
		e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
		e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
		e2eChartHTMLPath := filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals%s.html", timeSuffix))
		if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
			errors = append(errors, err)
		}
	} else {
		errors = append(errors, err)
	}

	// write out the current state of resources that we explicitly tracked.
	resourcesMap := monitor.CurrentResourceState()
	for resourceType, instanceMap := range resourcesMap {
		targetFile := fmt.Sprintf("resource-%s%s.zip", resourceType, timeSuffix)
		if err := monitorserialization.InstanceMapToFile(filepath.Join(artifactDir, targetFile), resourceType, instanceMap); err != nil {
			errors = append(errors, err)
		}
	}

	if _, err := getGraphableMetricsSeries(ctx, restConfig, startTime); err != nil {
		errors = append(errors, err)
	}

	return utilerrors.NewAggregate(errors)
}

// selectedMetricsQueries is a manually curated list of commonly useful metrics for a given area
var selectedMetricsQueriesByArea = map[string][]curatedQuery{
	"etcd": {
		{name: "etcd-fsync-duration", query: `histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (instance, le))`},
	},
}

type curatedQuery struct {
	name  string
	query string
}

// this should return a struct that is easy to serialize into the correct json format for graph datapoints
func getGraphableMetricsSeries(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.EventInterval, error) {
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	errors := []error{}
	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  10 * time.Second,
	}
	for area, queries := range selectedMetricsQueriesByArea {
		for _, query := range queries {
			metrics, warningsForQuery, err := prometheusClient.QueryRange(ctx, query.query, timeRange)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			if len(warningsForQuery) > 0 {
				fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
			}
			fmt.Fprintf(os.Stderr, "#### for %q, got \n\n%v\n\n", area, metrics.String())

			_, err = createGraphableMetricsJSON(ctx, metrics, startTime)
			if err != nil {
				errors = append(errors, err)
				continue
			}
		}
	}

	return nil, utilerrors.NewAggregate(errors)
}

func createGraphableMetricsJSON(ctx context.Context, metrics prometheustypes.Value, startTime time.Time) ([]monitorapi.EventInterval, error) {
	ret := []monitorapi.EventInterval{}

	switch {
	case metrics.Type() == prometheustypes.ValMatrix:
		matrixAlert := metrics.(prometheustypes.Matrix)
		for _, alert := range matrixAlert {
			alertName := alert.Metric[prometheustypes.AlertNameLabel]
			// don't skip Watchdog because gaps in watchdog are noteworthy, unexpected, and they do happen.
			//if alertName == "Watchdog" {
			//	continue
			//}
			// many pending alerts we just don't care about
			if alert.Metric["alertstate"] == "pending" {
				if pendingAlertsToIgnoreForIntervals.Has(string(alertName)) {
					continue
				}
			}

			locator := "alert/" + alertName
			if node := alert.Metric["instance"]; len(node) > 0 {
				locator += " node/" + node
			}
			if namespace := alert.Metric["namespace"]; len(namespace) > 0 {
				locator += " ns/" + namespace
			}
			if pod := alert.Metric["pod"]; len(pod) > 0 {
				locator += " pod/" + pod
			}
			if container := alert.Metric["container"]; len(container) > 0 {
				locator += " container/" + container
			}

			alertIntervalTemplate := monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Locator: string(locator),
					Message: alert.Metric.String(),
				},
			}
			switch {
			// as I understand it, pending alerts are cases where the conditions except for "how long has been happening"
			// are all met.  Pending alerts include what level the eventual alert will be, but they are not errors in and
			// of themselves.  They are you useful to show in time to find patterns of "X fails concurrent with Y"
			case alert.Metric["alertstate"] == "pending":
				alertIntervalTemplate.Level = monitorapi.Info

			case alert.Metric["severity"] == "warning":
				alertIntervalTemplate.Level = monitorapi.Warning
			case alert.Metric["severity"] == "critical":
				alertIntervalTemplate.Level = monitorapi.Error
			case alert.Metric["severity"] == "info":
				alertIntervalTemplate.Level = monitorapi.Info
			default:
				alertIntervalTemplate.Level = monitorapi.Error
			}

			var alertStartTime *time.Time
			var lastTime *time.Time
			for _, currValue := range alert.Values {
				currTime := currValue.Timestamp.Time()
				if alertStartTime == nil {
					alertStartTime = &currTime
				}
				if lastTime == nil {
					lastTime = &currTime
				}
				// if it has been less than five seconds since we saw this, consider it the same interval and check
				// the next time.
				if math.Abs(currTime.Sub(*lastTime).Seconds()) < (5 * time.Second).Seconds() {
					lastTime = &currTime
					continue
				}

				// if it has been more than five seconds, consider this the start of a new occurrence and add the interval
				currAlertInterval := alertIntervalTemplate // shallow copy
				currAlertInterval.From = *alertStartTime
				currAlertInterval.To = *lastTime
				ret = append(ret, currAlertInterval)

				// now reset the tracking
				alertStartTime = &currTime
				lastTime = nil
			}

			currAlertInterval := alertIntervalTemplate // shallow copy
			currAlertInterval.From = *alertStartTime
			currAlertInterval.To = *lastTime
			ret = append(ret, currAlertInterval)
		}

	default:
		ret = append(ret, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: "alert/all",
				Message: fmt.Sprintf("unhandled type: %v", metrics.Type()),
			},
			From: startTime,
			To:   time.Now(),
		})
	}

	return ret, nil
}
