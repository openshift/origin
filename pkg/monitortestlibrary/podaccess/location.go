package podaccess

import (
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type NonUniquePodKey struct {
	Namespace string
	Name      string
}

type PodKey struct {
	Namespace string
	Name      string
	UID       string
}

func NonUniquePodToNode(intervals monitorapi.Intervals) map[NonUniquePodKey]string {
	ret := map[NonUniquePodKey]string{}
	for _, interval := range intervals {
		if !strings.Contains(interval.Locator, "pod/") {
			continue
		}

		pod := monitorapi.PodFrom(interval.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		node, _ := monitorapi.NodeFromLocator(interval.Locator)
		if len(node) == 0 {
			continue
		}

		key := NonUniquePodKey{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}
		ret[key] = node

	}

	return ret
}

func NonUniqueEtcdMemberToPod(intervals monitorapi.Intervals) map[string]NonUniquePodKey {
	ret := map[string]NonUniquePodKey{}
	for _, interval := range intervals {
		if !strings.Contains(interval.Locator, "pod/") {
			continue
		}
		if !strings.Contains(interval.Message, "local-member-id/") {
			continue
		}

		pod := monitorapi.PodFrom(interval.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		memberName := monitorapi.AnnotationsFromMessage(interval.Message)[monitorapi.AnnotationEtcdLocalMember]
		if len(memberName) == 0 {
			continue
		}

		val := NonUniquePodKey{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}
		ret[memberName] = val

	}

	return ret
}

func UniquePodToNode(intervals monitorapi.Intervals) map[PodKey]string {
	ret := map[PodKey]string{}
	for _, interval := range intervals {
		pod := monitorapi.PodFrom(interval.Locator)
		if len(pod.UID) == 0 {
			continue
		}
		node, _ := monitorapi.NodeFromLocator(interval.Locator)
		if len(node) == 0 {
			continue
		}

		key := PodKey{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			UID:       pod.UID,
		}
		ret[key] = node

	}

	return ret

}
