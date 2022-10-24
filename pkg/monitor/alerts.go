package monitor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func WhenWasAlertFiring(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName, namespace string) ([]monitorapi.EventInterval, error) {
	return whenWasAlertInState(ctx, prometheusClient, startTime, alertName, "firing", namespace)
}

func WhenWasAlertPending(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName, namespace string) ([]monitorapi.EventInterval, error) {
	return whenWasAlertInState(ctx, prometheusClient, startTime, alertName, "pending", namespace)
}

func whenWasAlertInState(ctx context.Context, prometheusClient prometheusv1.API, startTime time.Time, alertName, alertState, namespace string) ([]monitorapi.EventInterval, error) {
	if alertState != "pending" && alertState != "firing" {
		return nil, fmt.Errorf("unrecognized alertState: %v", alertState)
	}

	// Prometheus has a hardcoded maximum resolution of 11,000 points per timeseries.  The "Step"
	// used to be 1 second but if a query exceeded 3 hours, this query would fail.  A resolution of
	// 2 seconds is fine because the query will work for up to 6 hours and ALERTS change every 30s
	// at most.
	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  2 * time.Second,
	}
	query := ""
	switch {
	case len(namespace) == 0:
		query = fmt.Sprintf(`ALERTS{alertstate="%s",alertname=%q}`, alertState, alertName)
	case namespace == platformidentification.NamespaceOther:
		query = fmt.Sprintf(`ALERTS{alertstate="%s",alertname=%q}`, alertState, alertName)
	default:
		query = fmt.Sprintf(`ALERTS{alertstate="%s",alertname=%q,namespace=%q}`, alertState, alertName, namespace)
	}

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

	if namespace == platformidentification.NamespaceOther {
		ret = monitorapi.Intervals(ret).Filter(func(eventInterval monitorapi.EventInterval) bool {
			namespace := monitorapi.NamespaceFromLocator(eventInterval.Locator)
			return !platformidentification.KnownNamespaces.Has(namespace)
		})
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

	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, "openshift-monitoring", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return []monitorapi.EventInterval{}, nil
	}

	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	// Ensure that all Thanos queriers are connected to all Prometheus sidecars
	// before fetching the alerts. This avoids retrieving partial data
	// (possibly with gaps) when after an upgrade, one of the Prometheus
	// sidecars hasn't been reconnected yet to the Thanos queriers.
	if err = wait.PollImmediateWithContext(ctx, 5*time.Second, 5*time.Minute, func(context.Context) (bool, error) {
		v, warningsForQuery, err := prometheusClient.Query(ctx, `min(count by(pod) (thanos_store_nodes_grpc_connections{store_type="sidecar"})) == min(kube_statefulset_replicas{statefulset="prometheus-k8s"})`, time.Time{})
		if err != nil {
			return false, err
		}

		if len(warningsForQuery) > 0 {
			fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
		}

		if v.Type() != prometheustypes.ValVector {
			return false, fmt.Errorf("expecting a vector type, got %q", v.Type().String())
		}

		if len(v.(prometheustypes.Vector)) == 0 {
			fmt.Printf("#### at least one Prometheus sidecar isn't ready\n")
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("Thanos queriers not connected to all Prometheus sidecars: %w", err)
	}

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  2 * time.Second,
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

	// firing alerts trump pending alerts, so if the alerts will overlap when we render, then we want to have pending
	// broken up by firing, so the alert should not be listed as pending at the same time as it is firing in our intervals.
	pendingAlerts = blackoutEvents(pendingAlerts, firingAlerts)

	ret := []monitorapi.EventInterval{}
	ret = append(ret, firingAlerts...)
	ret = append(ret, pendingAlerts...)

	return ret, nil
}

// blackoutEvents filters startingEvents and rewrites into potentially multiple events to avoid overlap with the blackoutWindows.
// For instance, if startingEvents for locator/foo covers 1:00-1:45 and blackoutWindows for locator/Foo covers 1:10-1:15 and 1:40-1:50
// the return has locator/foo 1:00-1:10, 1:15-1:40.
func blackoutEvents(startingEvents, blackoutWindows []monitorapi.EventInterval) []monitorapi.EventInterval {
	ret := []monitorapi.EventInterval{}

	blackoutsByLocator := indexByLocator(blackoutWindows)
	for i := range startingEvents {
		startingEvent := startingEvents[i]
		blackouts := blackoutsByLocator[startingEvent.Locator]
		if len(blackouts) == 0 {
			ret = append(ret, startingEvent)
			continue
		}

		relatedBlackouts := nonOverlappingBlackoutWindowsFromEvents(blackouts)
		currStartTime := startingEvent.From
		maxEndTime := startingEvent.To
		for i, currBlackout := range relatedBlackouts {
			if currBlackout.To.Before(currStartTime) { // too early, does not apply
				continue
			}
			if currBlackout.From.After(maxEndTime) { // too late, does not apply and we're done
				break
			}
			var nextBlackout *blackoutWindow
			if nextIndex := i + 1; nextIndex < len(relatedBlackouts) {
				nextBlackout = &relatedBlackouts[nextIndex]
			}

			switch {
			case currBlackout.From.Before(currStartTime) || currBlackout.From == currStartTime:
				// if the blackoutEvent is before the currentStartTime, then the new startTime will be when this blackout ends
				eventNext := startingEvent
				eventNext.From = currBlackout.To
				if nextBlackout != nil && nextBlackout.From.Before(maxEndTime) {
					eventNext.To = nextBlackout.From
				} else {
					eventNext.To = maxEndTime
				}
				currStartTime = eventNext.To
				if eventNext.From != eventNext.To && eventNext.From.Before(eventNext.To) {
					ret = append(ret, eventNext)
				}

				// if we're at the end of the blackout list
				if nextBlackout == nil {
					eventNext = startingEvent
					eventNext.From = currStartTime
					eventNext.To = maxEndTime
					currStartTime = eventNext.To
					if eventNext.From != eventNext.To && eventNext.From.Before(eventNext.To) {
						ret = append(ret, eventNext)
					}
				}

			case currBlackout.To.After(maxEndTime) || currBlackout.To == maxEndTime:
				// this should be the last blackout that applies to us, because all the other ones will start *after* this .To
				// if the blackoutEvent ends after the maxEndTime, then the new maxEndTime will be when this blackout starts
				eventNext := startingEvent
				eventNext.From = currStartTime
				eventNext.To = currBlackout.From
				currStartTime = eventNext.To
				if eventNext.From != eventNext.To && eventNext.From.Before(eventNext.To) {
					ret = append(ret, eventNext)
				}

			default:
				// if we're here, then the blackout is in the middle of our overall timeframe
				eventNext := startingEvent
				eventNext.From = currStartTime
				eventNext.To = currBlackout.From
				currStartTime = currBlackout.To
				if eventNext.From != eventNext.To && eventNext.From.Before(eventNext.To) {
					ret = append(ret, eventNext)
				}

				// if we're at the end of the blackout list
				if nextBlackout == nil {
					eventNext = startingEvent
					eventNext.From = currStartTime
					eventNext.To = maxEndTime
					currStartTime = eventNext.To
					if eventNext.From != eventNext.To && eventNext.From.Before(eventNext.To) {
						ret = append(ret, eventNext)
					}
				}
			}

			// we're done
			if !currStartTime.Before(maxEndTime) {
				break
			}
		}

	}

	sort.Sort(monitorapi.Intervals(ret))
	return ret
}

type blackoutWindow struct {
	From time.Time
	To   time.Time
}

func nonOverlappingBlackoutWindowsFromEvents(blackoutWindows []monitorapi.EventInterval) []blackoutWindow {
	sort.Sort(monitorapi.Intervals(blackoutWindows))

	ret := []blackoutWindow{}

	for _, sourceWindow := range blackoutWindows {
		if len(ret) == 0 {
			ret = append(ret, blackoutWindow{
				From: sourceWindow.From,
				To:   sourceWindow.To,
			})
			continue
		}

		newRet := make([]blackoutWindow, len(ret))
		copy(newRet, ret)

		for j := range ret {
			resultWindow := ret[j]

			switch {
			case sourceWindow.From.After(resultWindow.From) && sourceWindow.To.Before(resultWindow.To):
				// strictly smaller, the source window can be ignored

			case sourceWindow.From.After(resultWindow.To):
				// too late, does not overlap add the source
				newRet = append(newRet, blackoutWindow{
					From: sourceWindow.From,
					To:   sourceWindow.To,
				})

			case sourceWindow.To.Before(resultWindow.From):
				// too early, does not overlap
				newRet = append(newRet, blackoutWindow{
					From: sourceWindow.From,
					To:   sourceWindow.To,
				})

			case sourceWindow.From.Before(resultWindow.From) && sourceWindow.To.After(resultWindow.To):
				// strictly larger, the new source window times should overwrite
				resultWindow.From = sourceWindow.From
				resultWindow.To = sourceWindow.To
				newRet[j] = resultWindow

			case sourceWindow.From.Before(resultWindow.From):
				// the sourceWindow starts before the resultWindow and to is somewhere during, the window should start earlier
				resultWindow.From = sourceWindow.From
				newRet[j] = resultWindow

			case sourceWindow.To.After(resultWindow.To):
				// the sourceWindow ends after the resultWindow and from is somewhere during, the window should end later
				resultWindow.To = sourceWindow.To
				newRet[j] = resultWindow

			default:
				// let's hope we don't do anything here
			}
		}

		ret = newRet
	}

	return ret
}

func indexByLocator(events []monitorapi.EventInterval) map[string][]monitorapi.EventInterval {
	ret := map[string][]monitorapi.EventInterval{}
	for i := range events {
		event := events[i]
		ret[event.Locator] = append(ret[event.Locator], event)
	}
	return ret
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
