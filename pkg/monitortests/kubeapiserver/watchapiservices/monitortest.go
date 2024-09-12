package watchapiservices

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
)

type machineWatcher struct {
}

func NewAPIServiceWatcher() monitortestframework.MonitorTest {
	return &machineWatcher{}
}

func (w *machineWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	apiserviceClient, err := apiregistrationv1client.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startAPIServiceMonitoring(ctx, recorder, apiserviceClient)

	return nil
}

func (w *machineWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*machineWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	allAPIServiceChanges := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.APIServiceCreated ||
			(eventInterval.Message.Reason == monitorapi.APIServiceConditionChanged && eventInterval.Message.Annotations[monitorapi.AnnotationCondition] == "Available") ||
			eventInterval.Message.Reason == monitorapi.APIServiceDeletedInAPI {
			return true
		}
		return false
	})

	apiserviceToChanges := map[string][]monitorapi.Interval{}
	for _, apiserviceChange := range allAPIServiceChanges {
		apiservice := apiserviceChange.Locator.Keys[monitorapi.LocatorAPIServiceKey]
		apiserviceToChanges[apiservice] = append(apiserviceToChanges[apiservice], apiserviceChange)
	}

	for _, allAPIServiceChanges := range apiserviceToChanges {
		previousChangeTime := time.Time{}
		createdIntervals := monitorapi.Intervals(allAPIServiceChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.APIServiceCreated
		})
		if len(createdIntervals) > 0 {
			previousChangeTime = createdIntervals[0].From
		}
		apiserviceLocator := monitorapi.Locator{}
		previousHumanMesage := ""
		lastAvailableStatus := ""

		availableConditionChanges := monitorapi.Intervals(allAPIServiceChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.APIServiceConditionChanged && eventInterval.Message.Annotations[monitorapi.AnnotationCondition] == "Available"
		})
		for _, availableConditionChange := range availableConditionChanges {
			previousStatus := availableConditionChange.Message.Annotations[monitorapi.AnnotationPreviousStatus]
			reason := monitorapi.APIServiceUnavailable
			intervalLevel := monitorapi.Error
			if previousStatus == "True" {
				reason = monitorapi.APIServiceAvailable
				intervalLevel = monitorapi.Info
			} else if previousStatus != "False" {
				reason = monitorapi.APIServiceUnknown
				intervalLevel = monitorapi.Warning
			}
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, intervalLevel).
					Locator(availableConditionChange.Locator).
					Message(monitorapi.NewMessage().Reason(reason).
						Constructed(monitorapi.ConstructionOwnerAPIServiceLifecycle).
						HumanMessage(previousHumanMesage)).
					Display().
					Build(previousChangeTime, availableConditionChange.From),
			)
			previousChangeTime = availableConditionChange.From
			lastAvailableStatus = availableConditionChange.Message.Annotations[monitorapi.AnnotationStatus]
			previousHumanMesage = availableConditionChange.Message.HumanMessage
			apiserviceLocator = availableConditionChange.Locator
		}

		deletionTime := time.Now()
		deletedIntervals := monitorapi.Intervals(allAPIServiceChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.APIServiceDeletedInAPI
		})
		if len(deletedIntervals) > 0 {
			deletionTime = deletedIntervals[0].To
		}
		if len(lastAvailableStatus) > 0 {
			reason := monitorapi.APIServiceUnavailable
			intervalLevel := monitorapi.Error
			if lastAvailableStatus == "True" {
				reason = monitorapi.APIServiceAvailable
				intervalLevel = monitorapi.Info
			} else if lastAvailableStatus != "False" {
				reason = monitorapi.APIServiceUnknown
				intervalLevel = monitorapi.Warning
			}
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, intervalLevel).
					Locator(apiserviceLocator).
					Message(monitorapi.NewMessage().Reason(reason).
						Constructed(monitorapi.ConstructionOwnerAPIServiceLifecycle).
						HumanMessage(previousHumanMesage)).
					Display().
					Build(previousChangeTime, deletionTime),
			)
		}
	}

	return constructedIntervals, nil
}

func (*machineWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	allOpenShiftAPIServices := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Locator.Type != monitorapi.LocatorTypeAPIService {
			return false
		}
		if !strings.HasPrefix(eventInterval.Locator.Keys[monitorapi.LocatorNamespaceKey], "openshift-") {
			return false
		}
		return true
	})
	unavailableOpenShiftAPIServices := allOpenShiftAPIServices.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason != monitorapi.APIServiceUnavailable {
			return false
		}
		return true
	})

	allOpenShiftAPIServiceNamespaces := sets.Set[string]{}
	for _, apiserviceInterval := range allOpenShiftAPIServices {
		allOpenShiftAPIServiceNamespaces.Insert(apiserviceInterval.Locator.Keys[monitorapi.LocatorNamespaceKey])
	}

	unavailableIntervalsByNamespace := map[string]monitorapi.Intervals{}
	for _, unavailableInterval := range unavailableOpenShiftAPIServices {
		namespace := unavailableInterval.Locator.Keys[monitorapi.LocatorNamespaceKey]
		unavailableIntervalsByNamespace[namespace] = append(unavailableIntervalsByNamespace[namespace], unavailableInterval)
	}

	for _, namespace := range sets.List(allOpenShiftAPIServiceNamespaces) {
		testName := fmt.Sprintf("APIServices in ns/%s must always have endpoints", namespace)
		failures := []string{}
		unavailableIntervals := unavailableIntervalsByNamespace[namespace]
		for _, unavailableInterval := range unavailableIntervals {
			failures = append(failures, unavailableInterval.Message.HumanMessage)
		}
		if len(failures) > 0 {
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: fmt.Sprintf("went unavailable %d times", len(failures)),
						Output:  strings.Join(failures, "\n"),
					},
					SystemOut: "sysout",
					SystemErr: "syserr",
				},
			)
		} else {
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
				},
			)
		}

	}

	return ret, nil
}

func (*machineWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*machineWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
