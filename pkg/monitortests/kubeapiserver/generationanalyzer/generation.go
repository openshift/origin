package generationanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startGenerationMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client kubernetes.Interface) {
	objChangeFns := []func(obj, oldObj metav1.Object) []monitorapi.Interval{
		func(obj, oldObj metav1.Object) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			var generation int64
			var name string
			if oldObj != nil {
				name = oldObj.GetName()
				generation = oldObj.GetGeneration()
			}
			if obj != nil {
				name = obj.GetName()
				generation = obj.GetGeneration()
			}

			if generation > maxGenerationAllowed {
				intervals = append(intervals, getInterval(name, generation, obj, oldObj))
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

func toCreateFns(objUpdateFns []func(obj, oldObj metav1.Object) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(metav1.Object), nil)
		})
	}

	return ret
}

func toUpdateFns(objUpdateFns []func(obj, oldObj metav1.Object) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(metav1.Object), nil)
			}
			return fn(obj.(metav1.Object), oldObj.(metav1.Object))
		})
	}

	return ret
}

func toDeleteFns(objUpdateFns []func(obj, oldObj metav1.Object) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range objUpdateFns {
		fn := objUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(nil, obj.(metav1.Object))
		})
	}
	return ret
}

func getInterval(name string, generation int64, obj, oldObj metav1.Object) monitorapi.Interval {
	var msg string
	var locator monitorapi.Locator

	validObj := obj
	if validObj == nil {
		validObj = oldObj
	}

	switch validObj.(type) {
	case *appsv1.Deployment:
		locator = monitorapi.NewLocator().DeploymentFromName(name)
		msg = fmt.Sprintf("Deployment generation too high: %d", generation)
	case *appsv1.DaemonSet:
		locator = monitorapi.NewLocator().DaemonSetFromName(name)
		msg = fmt.Sprintf("DaemonSet generation too high: %d", generation)
	case *appsv1.StatefulSet:
		locator = monitorapi.NewLocator().StatefulSetFromName(name)
		msg = fmt.Sprintf("StatefulSet generation too high: %d", generation)
	default:
		panic(fmt.Sprintf("invalid object type %T", validObj))
	}

	now := time.Now()
	return monitorapi.NewInterval(monitorapi.SourceGenerationMonitor, monitorapi.Info).
		Locator(locator).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.ReasonHighGeneration).
			HumanMessage(msg),
		).Build(now, now)
}
