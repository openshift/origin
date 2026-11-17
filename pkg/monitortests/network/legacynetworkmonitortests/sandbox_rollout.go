package legacynetworkmonitortests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// Allow a short OVN replacement and kubelet retry, but require positive recovery
// evidence so a persistent missing-CNI outage remains a failure.
const ovnSandboxRecoveryGracePeriod = time.Minute

// Network Progressing can clear before the final node's replacement OVN pod has
// restored CNI. Correlate that node's rollout and the affected pod's recovery
// instead of extending the cluster-wide operator excusal.
func sandboxFailureDuringOVNRollout(failure monitorapi.Interval, podEvents, ovnDeletions, operatorsProgressing monitorapi.Intervals) string {
	node := failure.Locator.Keys[monitorapi.LocatorNodeKey]
	if node == "" || failure.Locator.Keys[monitorapi.LocatorNamespaceKey] == "" ||
		failure.Locator.Keys[monitorapi.LocatorPodKey] == "" ||
		!strings.Contains(failure.Message.HumanMessage, "no CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?") {
		return ""
	}
	first, last, ok := sandboxEventOccurrences(failure)
	if !ok {
		return ""
	}

	var deletion *monitorapi.Interval
	for i := range ovnDeletions {
		candidate := &ovnDeletions[i]
		if candidate.Locator.Keys[monitorapi.LocatorNodeKey] == node &&
			!candidate.From.After(first) && (deletion == nil || candidate.From.After(deletion.From)) {
			deletion = candidate
		}
	}
	// Equal timestamps cannot establish ordering between independent watches.
	if deletion == nil || !first.After(deletion.From) {
		return ""
	}
	deadline := deletion.From.Add(ovnSandboxRecoveryGracePeriod)
	if failure.To.After(deadline) {
		return ""
	}
	progressing := false
	for _, operator := range operatorsProgressing {
		if operator.Source == monitorapi.SourceOperatorState &&
			operator.Locator.Keys[monitorapi.LocatorClusterOperatorKey] == "network" &&
			operator.Message.Annotations[monitorapi.AnnotationStatus] == "True" &&
			operator.From.Before(operator.To) &&
			!deletion.From.Before(operator.From) && !deletion.From.After(operator.To) {
			progressing = true
			break
		}
	}
	if !progressing {
		return ""
	}

	var recovery time.Time
	for _, event := range podEvents {
		if event.Message.Reason != "AddedInterface" ||
			!strings.HasPrefix(event.Message.HumanMessage, "Add eth0 [") ||
			!strings.HasSuffix(event.Message.HumanMessage, "] from ovn-kubernetes") {
			continue
		}
		if recoveryNode := event.Locator.Keys[monitorapi.LocatorNodeKey]; recoveryNode != "" && recoveryNode != node {
			continue
		}
		recoveredFrom, _, valid := sandboxEventOccurrences(event)
		if valid && recoveredFrom.After(last) && !event.To.After(deadline) &&
			(recovery.IsZero() || recoveredFrom.Before(recovery)) {
			recovery = recoveredFrom
		}
	}
	if recovery.IsZero() {
		return ""
	}
	for _, event := range ovnDeletions {
		if event.Locator.Keys[monitorapi.LocatorNodeKey] == node &&
			event.From.After(deletion.From) && !event.From.After(recovery) {
			return ""
		}
	}
	if !sandboxPodLifecycleCovers(podEvents, node, first, recovery) {
		return ""
	}
	return fmt.Sprintf("OVN rollout on node:%s recovered pod networking %.0fs after pod:%s began deletion",
		node, recovery.Sub(deletion.From).Seconds(), deletion.Locator.Keys[monitorapi.LocatorPodKey])
}

// KubeEvent intervals start at the last occurrence and may have a synthetic
// one-second duration. Use the annotations to include all aggregated occurrences.
func sandboxEventOccurrences(event monitorapi.Interval) (time.Time, time.Time, bool) {
	if event.Source != monitorapi.SourceKubeEvent {
		return time.Time{}, time.Time{}, false
	}
	first, firstErr := time.Parse(time.RFC3339, event.Message.Annotations["firstTimestamp"])
	last, lastErr := time.Parse(time.RFC3339, event.Message.Annotations["lastTimestamp"])
	valid := firstErr == nil && lastErr == nil && !first.IsZero() && !last.IsZero() &&
		!last.Before(first) && last.Equal(event.From) && !event.To.Before(last)
	return first, last, valid
}

func sandboxPodLifecycleCovers(events monitorapi.Intervals, node string, first, recovery time.Time) bool {
	var uid string
	covered := false
	for _, event := range events {
		if event.Source != monitorapi.SourcePodMonitor && event.Source != monitorapi.SourcePodState {
			continue
		}
		eventUID := event.Locator.Keys[monitorapi.LocatorUIDKey]
		if event.Source == monitorapi.SourcePodMonitor && eventUID == "" {
			return false
		}
		if eventUID != "" {
			// KubeEvents omit the pod UID. Do not infer recovery across a reused
			// namespace/name, even if only one incarnation appears in the window.
			if uid != "" && uid != eventUID {
				return false
			}
			uid = eventUID
		}
		if eventNode := event.Locator.Keys[monitorapi.LocatorNodeKey]; eventNode != "" && eventNode != node {
			return false
		}
		switch event.Message.Reason {
		case monitorapi.PodReasonGracefulDeleteStarted, monitorapi.PodReasonForceDelete, monitorapi.PodReasonDeleted:
			if !event.From.After(recovery) {
				return false
			}
		case monitorapi.PodReasonScheduled:
			if scheduledNode := event.Message.Annotations[monitorapi.AnnotationNode]; scheduledNode != "" && scheduledNode != node {
				return false
			}
			if event.Source == monitorapi.SourcePodState && eventUID != "" &&
				event.Message.Annotations[monitorapi.AnnotationNode] == node &&
				!event.From.After(first) && !event.To.Before(recovery) {
				covered = true
			}
		}
	}
	return covered
}
