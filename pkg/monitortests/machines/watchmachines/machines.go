package watchmachines

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	machine "github.com/openshift/api/machine/v1beta1"
	machineClient "github.com/openshift/client-go/machine/clientset/versioned"
	machineinformerv1 "github.com/openshift/client-go/machine/informers/externalversions/machine/v1beta1"
	"k8s.io/client-go/tools/cache"
)

func startMachineMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client machineClient.Interface) {

	machinePhaseChangeFns := []func(machine, oldMachine *machine.Machine) []monitorapi.Interval{
		func(machine, oldMachine *machine.Machine) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			now := time.Now()
			oldHasPhase := oldMachine != nil && oldMachine.Status.Phase != nil
			newHasPhase := machine != nil && machine.Status.Phase != nil
			oldPhase := "<missing>"
			newPhase := "<missing>"
			if oldHasPhase {
				oldPhase = *oldMachine.Status.Phase
			}
			if newHasPhase {
				newPhase = *machine.Status.Phase
			}

			if oldPhase != newPhase {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
						Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhaseChanged).
							WithAnnotation(monitorapi.AnnotationPreviousPhase, oldPhase).
							WithAnnotation(monitorapi.AnnotationPhase, newPhase).
							HumanMessage(fmt.Sprintf("Machine phase changed from %s to %s", oldPhase, newPhase))).
						Build(now, now))
			}
			return intervals
		},
	}

	machineInformer := machineinformerv1.NewMachineInformer(client, "openshift-machine-api", time.Hour, nil)
	machineInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
			},
			DeleteFunc: func(obj interface{}) {
			},
			UpdateFunc: func(old, obj interface{}) {
				newMachine, newOk := obj.(*machine.Machine)
				if !newOk {
					return
				}
				oldMachine, oldOk := old.(*machine.Machine)
				if !oldOk {
					return
				}
				for _, fn := range machinePhaseChangeFns {
					m.AddIntervals(fn(newMachine, oldMachine)...)
				}
			},
		},
	)

	go machineInformer.Run(ctx.Done())
}
