package watchmachines

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	"k8s.io/apimachinery/pkg/fields"

	machine "github.com/openshift/api/machine/v1beta1"
	machineClient "github.com/openshift/client-go/machine/clientset/versioned"
	"k8s.io/client-go/tools/cache"
)

func startMachineMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client machineClient.Interface) {

	machinePhaseChangeFns := []func(machine, oldMachine *machine.Machine) []monitorapi.Interval{
		func(machine, oldMachine *machine.Machine) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			now := time.Now()
			if machine.Status.Phase != nil && oldMachine.Status.Phase != nil && *machine.Status.Phase != *oldMachine.Status.Phase {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
						Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhaseChanged).
							HumanMessage(fmt.Sprintf("Machine phase changed from %s to %s", *oldMachine.Status.Phase, *machine.Status.Phase))).
						Build(now, now))
			}
			return intervals
		},
	}

	nullFunc := []func(machine *machine.Machine) []monitorapi.Interval{
		func(oldMachine *machine.Machine) []monitorapi.Interval { return nil },
	}
	listWatch := cache.NewListWatchFromClient(client.Discovery().RESTClient(), "machines", "openshift-machine-api", fields.Everything())
	customStore := monitortestlibrary.NewMonitoringStore(
		"machines",
		toNullCreateFns(nullFunc),
		toUpdateFns(machinePhaseChangeFns),
		toDeleteFns(nullFunc),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &machine.Machine{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

func toNullCreateFns([]func(_ *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}
	return ret
}

func toDeleteFns(_ []func(pod *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}
	return ret
}
func toUpdateFns(podUpdateFns []func(machine, oldMachine *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range podUpdateFns {
		fn := podUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*machine.Machine), nil)
			}
			return fn(obj.(*machine.Machine), oldObj.(*machine.Machine))
		})
	}

	return ret
}
