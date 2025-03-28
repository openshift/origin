package alertanalyzer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
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

func fetchEventIntervalsForAllAlerts(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
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
	alerts, warningsForQuery, err := prometheusClient.QueryRange(ctx, `ALERTS{alertstate="firing"}`, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		fmt.Printf("#### warnings \n\t%v\n", strings.Join(warningsForQuery, "\n\t"))
	}

	firingAlerts, err := createEventIntervalsForAlerts(ctx, alerts, startTime)
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
	pendingAlerts, err := createEventIntervalsForAlerts(ctx, alerts, startTime)
	if err != nil {
		return nil, err
	}

	// firing alerts trump pending alerts, so if the alerts will overlap when we render, then we want to have pending
	// broken up by firing, so the alert should not be listed as pending at the same time as it is firing in our intervals.
	pendingAlerts = blackoutEvents(pendingAlerts, firingAlerts)

	ret := []monitorapi.Interval{}
	ret = append(ret, firingAlerts...)
	ret = append(ret, pendingAlerts...)

	return ret, nil
}

// blackoutEvents filters startingEvents and rewrites into potentially multiple events to avoid overlap with the blackoutWindows.
// For instance, if startingEvents for locator/foo covers 1:00-1:45 and blackoutWindows for locator/Foo covers 1:10-1:15 and 1:40-1:50
// the return has locator/foo 1:00-1:10, 1:15-1:40.
func blackoutEvents(startingEvents, blackoutWindows []monitorapi.Interval) []monitorapi.Interval {
	ret := []monitorapi.Interval{}

	blackoutsByLocator := indexByLocator(blackoutWindows)
	for i := range startingEvents {
		startingEvent := startingEvents[i]
		blackouts := blackoutsByLocator[startingEvent.Locator.OldLocator()]
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

func nonOverlappingBlackoutWindowsFromEvents(blackoutWindows []monitorapi.Interval) []blackoutWindow {
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

func indexByLocator(events []monitorapi.Interval) map[string][]monitorapi.Interval {
	ret := map[string][]monitorapi.Interval{}
	for i := range events {
		event := events[i]
		ret[event.Locator.OldLocator()] = append(ret[event.Locator.OldLocator()], event)
	}
	return ret
}

func createEventIntervalsForAlerts(ctx context.Context, alerts prometheustypes.Value, startTime time.Time) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case alerts.Type() == prometheustypes.ValMatrix:
		matrixAlert := alerts.(prometheustypes.Matrix)
		for _, alert := range matrixAlert {

			lb := monitorapi.NewLocator().AlertFromPromSampleStream(alert)

			var level monitorapi.IntervalLevel
			switch {
			// as I understand it, pending alerts are cases where the conditions except for "how long has been happening"
			// are all met.  Pending alerts include what level the eventual alert will be, but they are not errors in and
			// of themselves.  They are you useful to show in time to find patterns of "X fails concurrent with Y"
			case alert.Metric["alertstate"] == "pending":
				level = monitorapi.Info

			case alert.Metric["severity"] == "warning":
				level = monitorapi.Warning
			case alert.Metric["severity"] == "critical":
				level = monitorapi.Error
			case alert.Metric["severity"] == "info": // this case may not exist
				level = monitorapi.Info
			default:
				level = monitorapi.Error
			}
			msg := monitorapi.NewMessage().HumanMessage(alert.Metric.String())
			if len(string(alert.Metric["alertstate"])) > 0 {
				msg = msg.WithAnnotation(monitorapi.AnnotationAlertState, string(alert.Metric["alertstate"]))
			}
			if len(string(alert.Metric["severity"])) > 0 {
				msg = msg.WithAnnotation(monitorapi.AnnotationSeverity, string(alert.Metric["severity"]))
			}
			alertIntervalTemplate :=
				monitorapi.NewInterval(monitorapi.SourceAlert, level).
					Display().
					Locator(lb).
					Message(msg)

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
				ret = append(ret, alertIntervalTemplate.Build(*alertStartTime, *lastTime))

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
			ret = append(ret, alertIntervalTemplate.Build(*alertStartTime, *lastTime))
		}

	default:
		logrus.WithField("type", alerts.Type()).Warning("unhandled prometheus alert type received in alert monitor")
	}

	return ret, nil
}
