package monitor

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func WhenWasAlertFiring(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName string) ([]monitorapi.EventInterval, error) {
	return whenWasAlertInState(ctx, prometheusClient, startTime, alertName, "firing")
}

func WhenWasAlertPending(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName string) ([]monitorapi.EventInterval, error) {
	return whenWasAlertInState(ctx, prometheusClient, startTime, alertName, "pending")
}

func whenWasAlertInState(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName, alertState string) ([]monitorapi.EventInterval, error) {
	if alertState != "pending" && alertState != "firing" {
		return nil, fmt.Errorf("unrecognized alertState: %v", alertState)
	}
	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  1 * time.Second,
	}
	query := fmt.Sprintf(`ALERTS{alertstate="%s",alertname=%q}`, alertState, alertName)
	alerts, warningsForQuery, err := prometheusClient.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
	}

	ret, err := CreateEventIntervalsForAlerts(ctx, alerts, startTime)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func FetchEventIntervalsForAllAlerts(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.EventInterval, error) {
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

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  1 * time.Second,
	}
	alerts, warningsForQuery, err := prometheusClient.QueryRange(ctx, `ALERTS{alertstate="firing"}`, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
	}

	firingAlerts, err := CreateEventIntervalsForAlerts(ctx, alerts, startTime)
	if err != nil {
		return nil, err
	}

	alerts, warningsForQuery, err = prometheusClient.QueryRange(ctx, `ALERTS{alertstate="pending"}`, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
	}
	pendingAlerts, err := CreateEventIntervalsForAlerts(ctx, alerts, startTime)
	if err != nil {
		return nil, err
	}

	ret := []monitorapi.EventInterval{}
	ret = append(ret, firingAlerts...)
	ret = append(ret, pendingAlerts...)

	return ret, nil
}

func CreateEventIntervalsForAlerts(ctx context.Context, alerts prometheustypes.Value, startTime time.Time) ([]monitorapi.EventInterval, error) {
	ret := []monitorapi.EventInterval{}

	switch {
	case alerts.Type() == prometheustypes.ValMatrix:
		matrixAlert := alerts.(prometheustypes.Matrix)
		for _, alert := range matrixAlert {
			alertName := alert.Metric[prometheustypes.AlertNameLabel]

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

			// now add the one for the last start time.  If we do not have a last time, it means we saw the start, but not
			// the end.  We don't know when this alert ended, but our threshold time from above is five seconds so we will
			// simply assign that here as "better than nothing"
			if lastTime == nil {
				t := alertStartTime.Add(5 * time.Second)
				lastTime = &t
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
				Message: fmt.Sprintf("unhandled type: %v", alerts.Type()),
			},
			From: startTime,
			To:   time.Now(),
		})
	}

	return ret, nil
}
