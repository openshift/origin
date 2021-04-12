package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startNodeMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	nodeChangeFns := []func(node, oldNode *corev1.Node) []monitorapi.Condition{
		func(node, oldNode *corev1.Node) []monitorapi.Condition {
			var conditions []monitorapi.Condition
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
						Message: fmt.Sprintf("condition/%s status/%s reason/%s changed", c.Type, c.Status, c.Reason),
					})
				}
			}
			if node.UID != oldNode.UID {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("node was deleted and recreated"),
				})
			}
			return conditions
		},
		func(node, oldNode *corev1.Node) []monitorapi.Condition {
			var conditions []monitorapi.Condition

			oldConfig := oldNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
			newConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
			oldDesired := oldNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
			newDesired := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]

			if newDesired != oldDesired {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("reason/MachineConfigChange config/%s config change requested", newDesired),
				})
			}
			if oldConfig != newConfig && newDesired == newConfig {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: fmt.Sprintf("reason/MachineConfigReached config/%s reached desired config", newDesired),
				})
			}
			return conditions
		},
	}

	nodeInformer := informercorev1.NewNodeInformer(client, time.Hour, nil)
	nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {},
			DeleteFunc: func(obj interface{}) {
				node, ok := obj.(*corev1.Node)
				if !ok {
					return
				}
				m.Record(monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: monitorapi.NodeLocator(node.Name),
					Message: "deleted",
				})
			},
			UpdateFunc: func(old, obj interface{}) {
				node, ok := obj.(*corev1.Node)
				if !ok {
					return
				}
				oldNode, ok := old.(*corev1.Node)
				if !ok {
					return
				}
				for _, fn := range nodeChangeFns {
					m.Record(fn(node, oldNode)...)
				}
			},
		},
	)

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
					Message: "node is not ready",
				})
			}
		}
		return conditions
	})

	go nodeInformer.Run(ctx.Done())
}
