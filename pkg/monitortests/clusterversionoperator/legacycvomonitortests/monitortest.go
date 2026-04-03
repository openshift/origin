package legacycvomonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type legacyMonitorTests struct {
	adminRESTConfig *rest.Config
}

func NewLegacyTests() monitortestframework.MonitorTest {
	return &legacyMonitorTests{}
}

func (w *legacyMonitorTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *legacyMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *legacyMonitorTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*legacyMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}
	ret = append(ret, intervalsFromEventsClusterVersionProgressing(startingIntervals)...)
	return ret, nil
}

var lastClusterVersionProgressingInterval *monitorapi.Interval

func intervalsFromEventsClusterVersionProgressing(intervals monitorapi.Intervals) monitorapi.Intervals {
	var ret monitorapi.Intervals

	for _, event := range intervals {
		if event.Source != monitorapi.SourceClusterOperatorMonitor {
			continue
		}
		if cvName := event.Locator.Keys[monitorapi.LocatorClusterVersionKey]; cvName != "version" {
			continue
		}
		currentCondition := monitorapi.GetOperatorConditionStatus(event)
		if currentCondition == nil {
			continue
		}

		if currentCondition.Type != configv1.OperatorProgressing {
			continue
		}

		if lastClusterVersionProgressingInterval != nil {
			ret = append(ret, monitorapi.NewInterval(monitorapi.SourceVersionState, monitorapi.Warning).
				Locator(event.Locator).
				Message(monitorapi.NewMessage().Reason(lastClusterVersionProgressingInterval.Message.Reason).
					HumanMessage(strings.Replace(lastClusterVersionProgressingInterval.Message.HumanMessage, "changed to ", "stayed in ", 1)).
					WithAnnotation(monitorapi.AnnotationCondition, string(configv1.OperatorProgressing)).
					WithAnnotation(monitorapi.AnnotationStatus, string(configv1.ConditionTrue))).
				Build(lastClusterVersionProgressingInterval.From, event.From),
			)
			lastClusterVersionProgressingInterval = nil
		}

		if currentCondition.Status == configv1.ConditionTrue &&
			strings.Contains(event.Message.HumanMessage, ProgressingWaitingCOsKey) {
			lastClusterVersionProgressingInterval = event.DeepCopy()
		}
	}
	return ret
}

func (w *legacyMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, testOperatorOSUpdateStaged(finalIntervals, w.adminRESTConfig)...)
	junits = append(junits, testOperatorOSUpdateStartedEventRecorded(finalIntervals, w.adminRESTConfig)...)

	isUpgrade := platformidentification.DidUpgradeHappenDuringCollection(finalIntervals, time.Time{}, time.Time{})
	topology, err := getControlPlaneTopology(w.adminRESTConfig)
	if err != nil {
		e2e.Logf("failed to get control plane topology: %v", err)
	}
	singleNode := topology == configv1.SingleReplicaTopologyMode

	if isUpgrade {
		upgradeFailed := hasUpgradeFailedEvent(finalIntervals)
		junits = append(junits, testUpgradeOperatorStateTransitions(finalIntervals, w.adminRESTConfig, topology, upgradeFailed)...)
		level, err := getUpgradeLevel(w.adminRESTConfig)
		if err != nil || level == unknownUpgradeLevel {
			return nil, fmt.Errorf("failed to determine upgrade level: %w", err)
		}
		junits = append(junits, testUpgradeOperatorProgressingStateTransitions(finalIntervals, level == patchUpgradeLevel, singleNode, upgradeFailed)...)
	} else {
		junits = append(junits, testStableSystemOperatorStateTransitions(finalIntervals, w.adminRESTConfig, singleNode)...)
	}

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}
