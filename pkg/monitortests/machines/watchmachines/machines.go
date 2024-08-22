package watchmachines

import (
	"context"
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
			if machine.Status.Phase != nil && oldMachine.Status.Phase != nil && *machine.Status.Phase != *oldMachine.Status.Phase {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Warning).
						Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhaseChanged).
							HumanMessage("changed")).
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
				machine, ok := obj.(*machine.Machine)
				if !ok {
					return
				}
				now := time.Now()
				i := monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
					Message(monitorapi.NewMessage().
						HumanMessage("deleted")).
					Build(now, now)
				m.AddIntervals(i)
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
