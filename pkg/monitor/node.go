package monitor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startNodeMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	nodeCreateFns := []func(node *corev1.Node) []monitorapi.Condition{}

	nodeChangeFns := []func(node, oldNode *corev1.Node) []monitorapi.Condition{
		func(node, oldNode *corev1.Node) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			roles := nodeRoles(node)

			for i := range node.Status.Conditions {
				c := &node.Status.Conditions[i]
				previous := findNodeCondition(oldNode.Status.Conditions, c.Type, i)
				if previous == nil {
					continue
				}
				if c.Status != previous.Status {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: monitorapi.NodeLocator(node.Name),
						Message: fmt.Sprintf("condition/%s status/%s reason/%s roles/%s changed", c.Type, c.Status, c.Reason, roles),
					})
				}
			}
			if node.UID != oldNode.UID {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("roles/%s node was deleted and recreated", roles),
				})
			}
			return conditions
		},
		func(node, oldNode *corev1.Node) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			roles := nodeRoles(node)

			oldConfig := oldNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
			newConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
			oldDesired := oldNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
			newDesired := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]

			if newDesired != oldDesired {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("reason/MachineConfigChange config/%s roles/%s config change requested", newDesired, roles),
				})
			}
			if oldConfig != newConfig && newDesired == newConfig {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("reason/MachineConfigReached config/%s roles/%s reached desired config", newDesired, roles),
				})
			}
			return conditions
		},
	}

	nodeDeleteFns := []func(node *corev1.Node) []monitorapi.Condition{
		func(node *corev1.Node) []monitorapi.Condition {
			return []monitorapi.Condition{
				{
					Level:   monitorapi.Warning,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("roles/%s deleted", nodeRoles(node)),
				},
			}
		},
	}

	listWatch := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	customStore := newMonitoringStore(
		"nodes",
		toNodeCreateFns(nodeCreateFns),
		toNodeUpdateFns(nodeChangeFns),
		toNodeDeleteFns(nodeDeleteFns),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Pod{}, customStore, 0)
	go reflector.Run(ctx.Done())

	nodeInformer := informercorev1.NewNodeInformer(client, time.Hour, nil)
	m.AddSampler(func(now time.Time) []*monitorapi.Condition {
		var conditions []*monitorapi.Condition
		for _, obj := range nodeInformer.GetStore().List() {
			node, ok := obj.(*corev1.Node)
			if !ok {
				continue
			}
			isReady := false
			if c := findNodeCondition(node.Status.Conditions, corev1.NodeReady, 0); c != nil {
				isReady = c.Status == corev1.ConditionTrue
			}
			if !isReady {
				conditions = append(conditions, &monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("roles/%s node is not ready", nodeRoles(node)),
				})
			}
		}
		return conditions
	})
	go nodeInformer.Run(ctx.Done())
}

func toNodeCreateFns(nodeCreateFns []func(node *corev1.Node) []monitorapi.Condition) []objCreateFunc {
	ret := []objCreateFunc{}

	for i := range nodeCreateFns {
		fn := nodeCreateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Condition {
			return fn(obj.(*corev1.Node))
		})
	}

	return ret
}

func toNodeDeleteFns(nodeDeleteFns []func(node *corev1.Node) []monitorapi.Condition) []objDeleteFunc {
	ret := []objDeleteFunc{}

	for i := range nodeDeleteFns {
		fn := nodeDeleteFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Condition {
			return fn(obj.(*corev1.Node))
		})
	}

	return ret
}

func toNodeUpdateFns(nodeUpdateFns []func(node, oldNode *corev1.Node) []monitorapi.Condition) []objUpdateFunc {
	ret := []objUpdateFunc{}

	for i := range nodeUpdateFns {
		fn := nodeUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Condition {
			if oldObj == nil {
				return fn(obj.(*corev1.Node), nil)
			}
			return fn(obj.(*corev1.Node), oldObj.(*corev1.Node))
		})
	}

	return ret
}

func nodeRoles(node *corev1.Node) string {
	const roleLabel = "node-role.kubernetes.io"
	var roles []string
	for label := range node.Labels {
		if strings.Contains(label, roleLabel) {
			roles = append(roles, label[len(roleLabel)+1:])
		}
	}

	sort.Strings(roles)
	return strings.Join(roles, ",")
}
