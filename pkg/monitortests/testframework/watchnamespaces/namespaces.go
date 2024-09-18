package watchnamespaces

import (
	"context"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// allObservedPlatformNamespaces contains a list of namespaces observed
var allObservedPlatformNamespaces = sets.Set[string]{}

func isPlatformNamespace(nsName string) bool {
	switch {
	case nsName == "default" || nsName == "kubernetes" || nsName == "openshift":
		return true

	case strings.HasPrefix(nsName, "openshift-must-gather-"):
		// we skip must-gather namespaces because the names vary by run and produce problems
		return false

	case strings.HasPrefix(nsName, "openshift-"):
		return true
	case strings.HasPrefix(nsName, "kube-"):
		return true
	default:
		return false
	}
}

func startNamespaceMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client kubernetes.Interface) {
	namespaceChangeFns := []func(namespace, oldNamespace *corev1.Namespace) []monitorapi.Interval{
		// this is first so namespace created shows up first when queried
		func(namespace, oldNamespace *corev1.Namespace) []monitorapi.Interval {
			// we only care about creates
			if oldNamespace != nil {
				return nil
			}
			if isPlatformNamespace(namespace.Name) {
				allObservedPlatformNamespaces.Insert(namespace.Name)
			}
			return nil
		},
	}

	listWatch := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "namespaces", "", fields.Everything())
	customStore := monitortestlibrary.NewMonitoringStore(
		"namespaces",
		toCreateFns(namespaceChangeFns),
		toUpdateFns(namespaceChangeFns),
		toDeleteFns(namespaceChangeFns),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Namespace{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

func toCreateFns(namespaceUpdateFns []func(namespace, oldNamespace *corev1.Namespace) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range namespaceUpdateFns {
		fn := namespaceUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(*corev1.Namespace), nil)
		})
	}

	return ret
}

func toDeleteFns(namespaceUpdateFns []func(namespace, oldNamespace *corev1.Namespace) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range namespaceUpdateFns {
		fn := namespaceUpdateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(nil, obj.(*corev1.Namespace))
		})
	}
	return ret
}
func toUpdateFns(namespaceUpdateFns []func(namespace, oldNamespace *corev1.Namespace) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range namespaceUpdateFns {
		fn := namespaceUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*corev1.Namespace), nil)
			}
			return fn(obj.(*corev1.Namespace), oldObj.(*corev1.Namespace))
		})
	}

	return ret
}
