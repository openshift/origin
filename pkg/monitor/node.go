package monitor

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startNodeMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	nodeChangeFns := []func(node, oldNode *corev1.Node) []Condition{
		func(node, oldNode *corev1.Node) []Condition {
			var conditions []Condition
			for i := range node.Status.Conditions {
				c := &node.Status.Conditions[i]
				previous := findNodeCondition(oldNode.Status.Conditions, c.Type, i)
				if previous == nil {
					continue
				}
				if c.Status != previous.Status {
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locateNode(node),
						Message: fmt.Sprintf("condition %s changed", c.Type),
					})
				}
			}
			if node.UID != oldNode.UID {
				conditions = append(conditions, Condition{
					Level:   Error,
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
				m.Record(Condition{
					Level:   Warning,
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

	m.AddSampler(func(now time.Time) []*Condition {
		var conditions []*Condition
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
				conditions = append(conditions, &Condition{
					Level:   Warning,
					Locator: locateNode(node),
					Message: "node is not ready",
				})
			}
		}
		return conditions
	})

	go nodeInformer.Run(ctx.Done())
}
