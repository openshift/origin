package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func IsNode(locator string) bool {
	_, ret := NodeFromLocator(locator)
	return ret
}

func NodeFromLocator(locator string) (string, bool) {
	if !strings.HasPrefix(locator, "node/") {
		return "", false
	}
	parts := strings.SplitN(locator, "/", 2)
	return parts[1], true
}

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
						Locator: locateNode(node),
						Message: fmt.Sprintf("condition/%s status/%s reason/%s changed", c.Type, c.Status, c.Reason),
					})
				}
			}
			if node.UID != oldNode.UID {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: locateNode(node),
					Message: fmt.Sprintf("node was deleted and recreated"),
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
				pod, ok := obj.(*corev1.Node)
				if !ok {
					return
				}
				m.Record(monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: locateNode(pod),
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
					Locator: locateNode(node),
					Message: "node is not ready",
				})
			}
		}
		return conditions
	})

	go nodeInformer.Run(ctx.Done())
}
