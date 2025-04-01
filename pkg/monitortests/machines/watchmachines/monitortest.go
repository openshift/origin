package watchmachines

import (
	"context"
	"fmt"
	machineClient "github.com/openshift/client-go/machine/clientset/versioned"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type machineWatcher struct {
}

func NewMachineWatcher() monitortestframework.MonitorTest {
	return &machineWatcher{}
}

func (w *machineWatcher) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	machineMonitoringClient, err := machineClient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startMachineMonitoring(ctx, recorder, machineMonitoringClient)
	return nil
}

func (w *machineWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *machineWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*machineWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	allMachineChanges := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.MachineCreated ||
			eventInterval.Message.Reason == monitorapi.MachinePhaseChanged ||
			eventInterval.Message.Reason == monitorapi.MachineDeletedInAPI {
			return true
		}
		return false
	})

	machineNameToChanges := map[string][]monitorapi.Interval{}
	for _, machinePhaseChange := range allMachineChanges {
		machineName := machinePhaseChange.Locator.Keys[monitorapi.LocatorMachineKey]
		machineNameToChanges[machineName] = append(machineNameToChanges[machineName], machinePhaseChange)
	}

	for _, allMachineChanges := range machineNameToChanges {
		previousChangeTime := time.Time{}
		createdIntervals := monitorapi.Intervals(allMachineChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.MachineCreated
		})
		if len(createdIntervals) > 0 {
			previousChangeTime = createdIntervals[0].From
		}
		machineLocator := monitorapi.Locator{}
		lastPhase := ""

		phaseChanges := monitorapi.Intervals(allMachineChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.MachinePhaseChanged
		})
		for _, phaseChange := range phaseChanges {
			previousPhase := phaseChange.Message.Annotations[monitorapi.AnnotationPreviousPhase]
			nodeName := phaseChange.Message.Annotations[monitorapi.AnnotationNode]
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(phaseChange.Locator).
					Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhase).
						Constructed(monitorapi.ConstructionOwnerMachineLifecycle).
						WithAnnotation(monitorapi.AnnotationPhase, previousPhase).
						WithAnnotation(monitorapi.AnnotationNode, nodeName).
						HumanMessage(fmt.Sprintf("Machine is in %q", previousPhase))).
					Display().
					Build(previousChangeTime, phaseChange.From),
			)
			previousChangeTime = phaseChange.From
			lastPhase = phaseChange.Message.Annotations[monitorapi.AnnotationPhase]
			machineLocator = phaseChange.Locator
		}

		deletionTime := time.Time{}
		nodeName := "unknown"
		deletedIntervals := monitorapi.Intervals(allMachineChanges).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.MachineDeletedInAPI
		})
		if len(deletedIntervals) > 0 {
			deletionTime = deletedIntervals[0].To
			nodeName = deletedIntervals[0].Message.Annotations[monitorapi.AnnotationNode]
		}
		if len(lastPhase) > 0 {
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(machineLocator).
					Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhase).
						Constructed(monitorapi.ConstructionOwnerMachineLifecycle).
						WithAnnotation(monitorapi.AnnotationPhase, lastPhase).
						WithAnnotation(monitorapi.AnnotationNode, nodeName).
						HumanMessage(fmt.Sprintf("Machine is in %q", lastPhase))).
					Display().
					Build(previousChangeTime, deletionTime),
			)
		}
	}

	return constructedIntervals, nil
}

func (*machineWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*machineWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*machineWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
