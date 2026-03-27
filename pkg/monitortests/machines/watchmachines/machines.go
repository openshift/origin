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

	machineChangeFns := []func(machine, oldMachine *machine.Machine) []monitorapi.Interval{
		// this is first so machine created shows up first when queried
		func(machine, oldMachine *machine.Machine) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if oldMachine != nil {
				return intervals
			}

			intervals = append(intervals,
				monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
					Message(monitorapi.NewMessage().Reason(monitorapi.MachineCreated).
						HumanMessage("Machine created")).
					Build(machine.ObjectMeta.CreationTimestamp.Time, machine.ObjectMeta.CreationTimestamp.Time))
			return intervals
		},

		func(machine, oldMachine *machine.Machine) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			now := time.Now()

			oldHasPhase := oldMachine != nil && oldMachine.Status.Phase != nil
			newHasPhase := machine != nil && machine.Status.Phase != nil
			oldPhase := "<missing>"
			newPhase := "<missing>"
			if oldHasPhase && *oldMachine.Status.Phase != "" {
				oldPhase = *oldMachine.Status.Phase
			}
			if newHasPhase && *machine.Status.Phase != "" {
				newPhase = *machine.Status.Phase
			}

			nodeName := "<unknown>"
			oldHasNodeRef := oldMachine != nil && oldMachine.Status.NodeRef != nil
			newHasNodeRef := machine != nil && machine.Status.NodeRef != nil
			if oldHasNodeRef {
				nodeName = oldMachine.Status.NodeRef.Name
			}
			if newHasNodeRef {
				nodeName = machine.Status.NodeRef.Name
			}

			machineName := "<missing>"
			if oldMachine != nil {
				machineName = oldMachine.Name
			}
			if machine != nil {
				machineName = machine.Name
			}

			if oldPhase != newPhase {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
						Locator(monitorapi.NewLocator().MachineFromName(machineName)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachinePhaseChanged).
							WithAnnotation(monitorapi.AnnotationPhase, newPhase).
							WithAnnotation(monitorapi.AnnotationPreviousPhase, oldPhase).
							WithAnnotation(monitorapi.AnnotationNode, nodeName).
							HumanMessage(fmt.Sprintf("Machine phase changed from %s to %s", oldPhase, newPhase))).
						Build(now, now))
			}
			return intervals
		},

		// this is last so machine deleted shows up last when queried
		func(machine, oldMachine *machine.Machine) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if machine == nil {
				return intervals
			}
			nodeName := "<unknown>"
			oldHasNodeRef := oldMachine != nil && oldMachine.Status.NodeRef != nil
			newHasNodeRef := machine.Status.NodeRef != nil
			if oldHasNodeRef {
				nodeName = oldMachine.Status.NodeRef.Name
			}

			if newHasNodeRef {
				nodeName = machine.Status.NodeRef.Name
			}

			now := time.Now()
			intervals = append(intervals,
				monitorapi.NewInterval(monitorapi.SourceMachine, monitorapi.Info).
					Locator(monitorapi.NewLocator().MachineFromName(machine.Name)).
					Message(monitorapi.NewMessage().Reason(monitorapi.MachineDeletedInAPI).
						WithAnnotation(monitorapi.AnnotationNode, nodeName).
						HumanMessage("Machine deleted")).
					Build(now, now))
			return intervals
		},
	}

	listWatch := cache.NewListWatchFromClient(client.MachineV1beta1().RESTClient(), "machines", "openshift-machine-api", fields.Everything())
	customStore := monitortestlibrary.NewMonitoringStore(
		"machines",
		toCreateFns(machineChangeFns),
		toUpdateFns(machineChangeFns),
		toDeleteFns(machineChangeFns),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &machine.Machine{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

func toCreateFns(machineUpdateFns []func(machine, oldMachine *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range machineUpdateFns {
		fn := machineUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(*machine.Machine), nil)
		})
	}

	return ret
}

func toDeleteFns(machineUpdateFns []func(machine, oldMachine *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range machineUpdateFns {
		fn := machineUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(nil, obj.(*machine.Machine))
		})
	}
	return ret
}
func toUpdateFns(machineUpdateFns []func(machine, oldMachine *machine.Machine) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range machineUpdateFns {
		fn := machineUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*machine.Machine), nil)
			}
			return fn(obj.(*machine.Machine), oldObj.(*machine.Machine))
		})
	}

	return ret
}
