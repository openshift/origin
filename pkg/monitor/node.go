package monitor

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
					Message: fmt.Sprintf("roles/%s deleted", nodeRoles(node)),
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
					Message: fmt.Sprintf("roles/%s node is not ready", nodeRoles(node)),
				})
			}
		}
		return conditions
	})

	go nodeInformer.Run(ctx.Done())
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
