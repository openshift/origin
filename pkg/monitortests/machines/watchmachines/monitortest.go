package watchmachines

import (
	"context"
	"fmt"
	"time"

	machineClient "github.com/openshift/client-go/machine/clientset/versioned"
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

func (w *machineWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	machineClient, err := machineClient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	startMachineMonitoring(ctx, recorder, machineClient)

	return nil
}

func (w *machineWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*machineWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	machinePhaseChangeIntervals := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.MachinePhaseChanged {
			return true
		}
		return false
	})

	machinePhaseChangesByMachine := map[string][]monitorapi.Interval{}
	for _, machinePhaseChangeInterval := range machinePhaseChangeIntervals {
		machineKey := machinePhaseChangeInterval.Locator.OldLocator()
		machinePhaseChangesByMachine[machineKey] = append(machinePhaseChangesByMachine[machineKey], machinePhaseChangeInterval)
	}

	for _, phaseChanges := range machinePhaseChangesByMachine {
		previousChangeTime := time.Time{}
		for _, phaseChange := range phaseChanges {
			previousPhase := phaseChange.Message.Annotations[monitorapi.AnnotationPreviousPhase]
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(phaseChange.Locator).
					Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhase).
						Constructed(monitorapi.ConstructionOwnerMachineLifecycle).
						WithAnnotation(monitorapi.AnnotationPhase, previousPhase).
						HumanMessage(fmt.Sprintf("Machine phase is %q", previousPhase))).
					Display().
					Build(previousChangeTime, phaseChange.To))

			previousChangeTime = phaseChange.To
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
