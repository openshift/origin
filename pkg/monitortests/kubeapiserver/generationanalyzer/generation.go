package generationanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	"k8s.io/apimachinery/pkg/fields"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Workload interface {
	GetData() *WorkloadData
}

type WorkloadWrapper struct {
	obj interface{}
}

type WorkloadData struct {
	Kind               string
	Name               string
	Namespace          string
	Generation         int64
	ObservedGeneration int64
	Locator            monitorapi.Locator
}

func (w *WorkloadWrapper) GetData() *WorkloadData {
	switch o := w.obj.(type) {
	case *appsv1.Deployment:
		return &WorkloadData{
			Kind:               o.Kind,
			Name:               o.Name,
			Namespace:          o.Namespace,
			Generation:         o.Generation,
			ObservedGeneration: o.Status.ObservedGeneration,
			Locator:            monitorapi.NewLocator().DeploymentFromName(o.Namespace, o.Name),
		}
	case *appsv1.DaemonSet:
		return &WorkloadData{
			Kind:               o.Kind,
			Name:               o.Name,
			Namespace:          o.Namespace,
			Generation:         o.Generation,
			ObservedGeneration: o.Status.ObservedGeneration,
			Locator:            monitorapi.NewLocator().DaemonSetFromName(o.Namespace, o.Name),
		}
	case *appsv1.StatefulSet:
		return &WorkloadData{
			Kind:               o.Kind,
			Name:               o.Name,
			Namespace:          o.Namespace,
			Generation:         o.Generation,
			ObservedGeneration: o.Status.ObservedGeneration,
			Locator:            monitorapi.NewLocator().StatefulSetFromName(o.Namespace, o.Name),
		}
	default:
		panic(fmt.Sprintf("unsupported type: %T", o))
	}
}

func startGenerationMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client kubernetes.Interface) {
	objChangeFns := []func(obj, oldObj Workload) []monitorapi.Interval{
		func(obj, oldObj Workload) []monitorapi.Interval {
			var intervals []monitorapi.Interval

			var objData *WorkloadData
			if obj != nil {
				objData = obj.GetData()
			} else {
				objData = oldObj.GetData()
			}

			// Regardless if this is a creation, deletion or update, check if the generation is too high
			if objData.Generation > maxGenerationAllowed {
				intervals = append(intervals, highGenerationInterval(objData.Locator, objData.Kind, objData.Generation))
			}

			// If this is an update, check if generation is increasing monotonically
			if oldObj != nil && obj != nil {
				objData := obj.GetData()
				oldObjData := oldObj.GetData()
				if objData.ObservedGeneration < oldObjData.ObservedGeneration {
					intervals = append(intervals, invalidGenerationInterval(
						objData.Locator,
						objData.Kind,
						objData.ObservedGeneration,
						oldObjData.ObservedGeneration,
					))
				}
			}

			return intervals
		},
	}

	listWatchDeployment := cache.NewListWatchFromClient(client.AppsV1().RESTClient(), "deployments", "", fields.Everything())
	listWatchDaemonSet := cache.NewListWatchFromClient(client.AppsV1().RESTClient(), "daemonsets", "", fields.Everything())
	listWatchStatefulSet := cache.NewListWatchFromClient(client.AppsV1().RESTClient(), "statefulsets", "", fields.Everything())

	customStoreDeployments := monitortestlibrary.NewMonitoringStore(
		"deployments",
		toCreateFns(objChangeFns),
		toUpdateFns(objChangeFns),
		toDeleteFns(objChangeFns),
		m,
		m,
	)

	customStoreDaemonSets := monitortestlibrary.NewMonitoringStore(
		"daemonsets",
		toCreateFns(objChangeFns),
		toUpdateFns(objChangeFns),
		toDeleteFns(objChangeFns),
		m,
		m,
	)

	customStoreStatefulSets := monitortestlibrary.NewMonitoringStore(
		"statefulsets",
		toCreateFns(objChangeFns),
		toUpdateFns(objChangeFns),
		toDeleteFns(objChangeFns),
		m,
		m,
	)

	reflectorDeployment := cache.NewReflector(listWatchDeployment, &appsv1.Deployment{}, customStoreDeployments, 0)
	reflectorDaemonSet := cache.NewReflector(listWatchDaemonSet, &appsv1.DaemonSet{}, customStoreDaemonSets, 0)
	reflectorStatefulSet := cache.NewReflector(listWatchStatefulSet, &appsv1.StatefulSet{}, customStoreStatefulSets, 0)

	go reflectorDeployment.Run(ctx.Done())
	go reflectorDaemonSet.Run(ctx.Done())
	go reflectorStatefulSet.Run(ctx.Done())
}

func toCreateFns(objUpdateFns []func(obj, oldObj Workload) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(&WorkloadWrapper{obj: obj}, nil)
		})
	}

	return ret
}

func toUpdateFns(objUpdateFns []func(obj, oldObj Workload) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(Workload), nil)
			}
			return fn(&WorkloadWrapper{obj: obj}, &WorkloadWrapper{obj: oldObj})
		})
	}

	return ret
}

func toDeleteFns(objUpdateFns []func(obj, oldObj Workload) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(nil, &WorkloadWrapper{obj: obj})
		})
	}
	return ret
}

func highGenerationInterval(locator monitorapi.Locator, kind string, generation int64) monitorapi.Interval {
	now := time.Now()
	return monitorapi.NewInterval(monitorapi.SourceGenerationMonitor, monitorapi.Info).
		Locator(locator).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.ReasonHighGeneration).
			HumanMessage(fmt.Sprintf("%s generation too high: %d", kind, generation)),
		).Build(now, now)
}

func invalidGenerationInterval(locator monitorapi.Locator, kind string, newGeneration, previousGeneration int64) monitorapi.Interval {
	now := time.Now()
	return monitorapi.NewInterval(monitorapi.SourceGenerationMonitor, monitorapi.Info).
		Locator(locator).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.ReasonInvalidGeneration).
			HumanMessage(fmt.Sprintf("new %s generation (%d) is higher than previous generation (%d)", kind, newGeneration, previousGeneration)),
		).Build(now, now)
}
