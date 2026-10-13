package watchnodes

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

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

		isReady := isNodeReady(node)
		wasReady := false
		if !isCreate {
			wasReady = isNodeReady(oldNode)
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
		// This function is added to help detect unexpected
		// node not ready.
		// We want to fail the monitor test if a node goes not ready
		// if it is unexpected.
		// Unexpected in this case means that it went not ready outside
		// of a MCO config update.
		func(node, oldNode *corev1.Node) []monitorapi.Interval {
			var intervals []monitorapi.Interval

			oldConfig := oldNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
			newDesired := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
			isOldNodeReady := isNodeReady(oldNode)
			isNewNodeReady := isNodeReady(node)
			isConfigTheSame := oldConfig == newDesired
			isNodeUnscheduable := node.Spec.Unschedulable

			now := time.Now()
			if isOldNodeReady && !isNewNodeReady && isConfigTheSame && !isNodeUnscheduable {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceUnexpectedReady, monitorapi.Error).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.NodeUnexpectedReadyReason).
							HumanMessage("unexpected node not ready")).
						Display().
						Build(now, now))
			}
			return intervals
		},
		// This case will create an interval if a node is unreachable but the node was not drained
		// This will be reported as a failed test.
		func(node, oldNode *corev1.Node) []monitorapi.Interval {
			var intervals []monitorapi.Interval

			isOldNodeUnReachable := doesNodeHaveUnreachableTaints(oldNode)
			isNewNodeUnReachable := doesNodeHaveUnreachableTaints(node)
			isNodeUnscheduable := node.Spec.Unschedulable

			now := time.Now()
			// always mark unreachable
			if !isOldNodeUnReachable && isNewNodeUnReachable {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceUnreachable, monitorapi.Info).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.NodeUnreachable).
							HumanMessage("node unreachable")).
						Display().
						Build(now, now))
			}

			// sometimes also create the unexpectedunreachable
			if !isOldNodeUnReachable && isNewNodeUnReachable && !isNodeUnscheduable {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceUnreachable, monitorapi.Error).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.NodeUnexpectedUnreachableReason).
							HumanMessage("unexpected node unreachable")).
						Display().
						Build(now, now))
			}
			return intervals
		},
		// Watch for node reporting DiskPressure and create a point in time interval if we see this.
		// We don't particularly care how long it was reported for, only that it happened.
		func(node, oldNode *corev1.Node) []monitorapi.Interval {
			var intervals []monitorapi.Interval

			var oldNodeDiskPressure bool
			if oldNode != nil {
				for _, c := range oldNode.Status.Conditions {
					if c.Type == corev1.NodeDiskPressure && c.Status == corev1.ConditionTrue {
						oldNodeDiskPressure = true
					}
				}
			}
			var newNodeDiskPressure bool
			if node != nil {
				for _, c := range node.Status.Conditions {
					if c.Type == corev1.NodeDiskPressure && c.Status == corev1.ConditionTrue {
						newNodeDiskPressure = true
					}
				}
			}
			now := time.Now()
			if !oldNodeDiskPressure && newNodeDiskPressure {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.NodeDiskPressure).
							HumanMessage("kubelet began reporting disk pressure")).
						Display().
						Build(now, now))
			}
			if oldNodeDiskPressure && !newNodeDiskPressure {
				intervals = append(intervals,
					monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Info).
						Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
						Message(monitorapi.NewMessage().Reason(monitorapi.NodeNoDiskPressure).
							HumanMessage("kubelet is not reporting disk pressure")).
						Display().
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
						Reason(monitorapi.NodeDeleted).
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
	const roleLabel = "node-role.kubernetes.io/"
	var roles []string
	for label := range node.Labels {
		if strings.Contains(label, roleLabel) {
			role := label[len(roleLabel):]
			if role == "" {
				logrus.Warningf("ignoring blank role label %s", roleLabel)
				continue
			}
			roles = append(roles, role)
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

func isNodeReady(node *corev1.Node) bool {
	isReady := false
	if node == nil {
		return isReady
	}
	if c := findNodeCondition(node.Status.Conditions, corev1.NodeReady, 0); c != nil {
		isReady = c.Status == corev1.ConditionTrue
	}
	return isReady
}

func doesNodeHaveUnreachableTaints(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	taints := node.Spec.Taints
	for _, val := range taints {
		if val.Key == corev1.TaintNodeUnreachable {
			return true
		}
	}
	return false
}

func reportUnexpectedNodeDownFailures(intervals monitorapi.Intervals, targetedReason monitorapi.IntervalReason) []string {
	// Get all the deleted machine phases
	machineDeletePhases := intervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason != monitorapi.MachinePhase {
			return false
		}
		if eventInterval.Message.Annotations[monitorapi.AnnotationPhase] == "Deleting" {
			return true
		}
		return false
	})

	// We need to build a map of node to machine name
	nodeNameToMachineName := map[string]string{}
	// Given the deleted machine, store the deleted intervals.
	machineNameToDeletePhases := map[string][]monitorapi.Interval{}
	for _, machineDeletePhase := range machineDeletePhases {
		machineName := machineDeletePhase.Locator.Keys[monitorapi.LocatorMachineKey]
		nodeName := machineDeletePhase.Message.Annotations[monitorapi.AnnotationNode]
		machineNameToDeletePhases[machineName] = append(machineNameToDeletePhases[machineName], machineDeletePhase)
		nodeNameToMachineName[nodeName] = machineName
	}
	unexpectedNodeUnreadies := intervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == targetedReason
	})

	// In cases of machine deletion, we are incorrectly detecting unexpected node not ready and unreachable.
	// We are filtering out deleted machine intervals
	var failures []string
	for _, unexpectedNodeUnready := range unexpectedNodeUnreadies {
		nodeName := unexpectedNodeUnready.Locator.Keys[monitorapi.LocatorNodeKey]
		machineNameForNode := nodeNameToMachineName[nodeName]

		machineDeletingIntervals := machineNameToDeletePhases[machineNameForNode]

		if !intervalStartDuring(unexpectedNodeUnready, machineDeletingIntervals) {
			failures = append(failures, fmt.Sprintf("%v - %v at from: %v - to: %v", unexpectedNodeUnready.Locator.OldLocator(), unexpectedNodeUnready.Message.OldMessage(), unexpectedNodeUnready.From, unexpectedNodeUnready.To))
		}
	}

	return failures
}
