package watchnodes

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startNodeMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client kubernetes.Interface) {
	nodeReadyFn := func(node, oldNode *corev1.Node) []monitorapi.Interval {
		isCreate := false
		if oldNode == nil {
			isCreate = true
		}

		isReady := false
		if c := findNodeCondition(node.Status.Conditions, corev1.NodeReady, 0); c != nil {
			isReady = c.Status == corev1.ConditionTrue
		}

		wasReady := false
		if !isCreate {
			if c := findNodeCondition(oldNode.Status.Conditions, corev1.NodeReady, 0); c != nil {
				wasReady = c.Status == corev1.ConditionTrue
			}

		}

		now := time.Now()
		switch {
		case isCreate && !isReady:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
					Message(monitorapi.NewMessage().Reason("NotReady").
						WithAnnotation(monitorapi.AnnotationRoles, nodeRoles(node)).
						HumanMessage("node is not ready")).Build(now, now),
			}

		case isCreate && isReady:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
					Message(monitorapi.NewMessage().Reason("Ready").
						WithAnnotation(monitorapi.AnnotationRoles, nodeRoles(node)).
						HumanMessage("node is ready")).Build(now, now),
			}

		case wasReady && !isReady:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
					Message(monitorapi.NewMessage().Reason("NotReady").
						WithAnnotation(monitorapi.AnnotationRoles, nodeRoles(node)).
						HumanMessage("node is not ready")).Build(now, now),
			}

		case !wasReady && isReady:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
					Message(monitorapi.NewMessage().Reason("Ready").
						WithAnnotation(monitorapi.AnnotationRoles, nodeRoles(node)).
						HumanMessage("node is ready")).Build(now, now),
			}
		}

		return nil
	}

	nodeAddFns := []func(node *corev1.Node) []monitorapi.Interval{
		func(node *corev1.Node) []monitorapi.Interval {
			return nodeReadyFn(node, nil)
		},
	}
	nodeChangeFns := []func(node, oldNode *corev1.Node) []monitorapi.Interval{
		nodeReadyFn,
		func(node, oldNode *corev1.Node) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			roles := nodeRoles(node)

			now := time.Now()
			for i := range node.Status.Conditions {
				c := &node.Status.Conditions[i]
				previous := findNodeCondition(oldNode.Status.Conditions, c.Type, i)
				if previous == nil {
					continue
				}
				if c.Status != previous.Status {
					intervals = append(intervals,
						monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
							Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
							Message(monitorapi.NewMessage().Reason(monitorapi.IntervalReason(c.Reason)).
								WithAnnotations(map[monitorapi.AnnotationKey]string{
									monitorapi.AnnotationRoles: roles,
								}).
								HumanMessage("changed")).
							Build(now, now))
				}
			}
			if node.UID != oldNode.UID {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Error).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().
							WithAnnotations(map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationRoles: roles,
							}).
							HumanMessage("node was deleted and recreated")).
						Build(now, now))
			}
			return intervals
		},
		func(node, oldNode *corev1.Node) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			roles := nodeRoles(node)

			oldConfig := oldNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
			newConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
			oldDesired := oldNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
			newDesired := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]

			now := time.Now()
			if newDesired != oldDesired {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Info).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachineConfigChangeReason).
							WithAnnotations(map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationRoles:  roles,
								monitorapi.AnnotationConfig: newDesired,
							}).
							HumanMessage("config change requested")).
						Build(now, now))
			}
			if oldConfig != newConfig && newDesired == newConfig {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Info).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.MachineConfigReachedReason).
							WithAnnotations(map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationRoles:  roles,
								monitorapi.AnnotationConfig: newDesired,
							}).
							HumanMessage("reached desired config")).
						Build(now, now))
			}
			return intervals
		},
	}

	nodeInformer := informercorev1.NewNodeInformer(client, time.Hour, nil)
	nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				node, ok := obj.(*corev1.Node)
				if !ok {
					return
				}
				for _, fn := range nodeAddFns {
					m.AddIntervals(fn(node)...)
				}
			},
			DeleteFunc: func(obj interface{}) {
				node, ok := obj.(*corev1.Node)
				if !ok {
					return
				}
				now := time.Now()
				i := monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
					Message(monitorapi.NewMessage().
						WithAnnotations(map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationRoles: nodeRoles(node),
						}).
						HumanMessage("deleted")).
					Build(now, now)
				m.AddIntervals(i)
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
					m.AddIntervals(fn(node, oldNode)...)
				}
			},
		},
	)

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

func findNodeCondition(status []corev1.NodeCondition, name corev1.NodeConditionType, position int) *corev1.NodeCondition {
	if position < len(status) {
		if status[position].Type == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Type == name {
			return &status[i]
		}
	}
	return nil
}
