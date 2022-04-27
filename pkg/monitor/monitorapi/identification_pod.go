package monitorapi

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func LocatePod(pod *corev1.Pod) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s uid/%s", pod.Namespace, pod.Name, pod.Spec.NodeName, pod.UID)
}

func ReasonFrom(message string) string {
	tokens := strings.Split(message, " ")
	annotations := map[string]string{}
	for _, curr := range tokens {
		if !strings.Contains(curr, "/") {
			continue
		}
		annotationTokens := strings.Split(curr, "/")
		annotations[annotationTokens[0]] = annotationTokens[1]
	}
	return annotations["reason"]
}

func ReasonedMessage(reason string, message ...string) string {
	return fmt.Sprintf("reason/%v %s", reason, strings.Join(message, "; "))
}

func ReasonedMessagef(reason, messageFormat string, a ...interface{}) string {
	return ReasonedMessage(reason, fmt.Sprintf(messageFormat, a...))
}

const (
	// PodIPReused means the same pod IP is in use by two pods at the same time.
	PodIPReused = "ReusedPodIP"
)
