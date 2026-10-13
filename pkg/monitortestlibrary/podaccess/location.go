package podaccess

import (
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
		if !interval.Locator.HasKey(monitorapi.LocatorPodKey) {
			continue
		}

		pod := monitorapi.PodFrom(interval.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		node, _ := interval.Locator.Keys[monitorapi.LocatorNodeKey]
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
		if interval.Source != monitorapi.SourceEtcdLog {
			continue
		}
		if !interval.Locator.HasKey(monitorapi.LocatorPodKey) {
			continue
		}
		if _, ok := interval.Message.Annotations[monitorapi.AnnotationEtcdLocalMember]; !ok {
			continue
		}

		pod := monitorapi.PodFrom(interval.Locator)
		if len(pod.Name) == 0 {
			continue
		}
		memberName := interval.Message.Annotations[monitorapi.AnnotationEtcdLocalMember]
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
