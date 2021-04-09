package monitorapi

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func E2ETestLocator(testName string) string {
	return fmt.Sprintf("e2e-test/%q", testName)
}

func IsE2ETest(locator string) bool {
	_, ret := E2ETestFromLocator(locator)
	return ret
}

func E2ETestFromLocator(locator string) (string, bool) {
	if !strings.HasPrefix(locator, "e2e-test/") {
		return "", false
	}
	parts := strings.SplitN(locator, "/", 2)
	quotedTestName := parts[1]
	testName, err := strconv.Unquote(quotedTestName)
	if err != nil {
		return "", false
	}
	return testName, true
}

func NodeLocator(testName string) string {
	return fmt.Sprintf("node/%v", testName)
}

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

func OperatorLocator(testName string) string {
	return fmt.Sprintf("clusteroperator/%v", testName)
}

func IsOperator(locator string) bool {
	_, ret := OperatorFromLocator(locator)
	return ret
}

func OperatorFromLocator(locator string) (string, bool) {
	if !strings.HasPrefix(locator, "clusteroperator/") {
		return "", false
	}
	parts := strings.SplitN(locator, "/", 2)
	return parts[1], true
}

func PodFromLocator(locator string) (namespace, name string, matches bool) {
	parts := strings.Split(locator, " ")

	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "ns/"):
			tokens := strings.SplitN(part, "/", 2)
			namespace = tokens[1]
		case strings.HasPrefix(part, "pod/"):
			tokens := strings.SplitN(part, "/", 2)
			name = tokens[1]
		}
	}
	if len(namespace) == 0 || len(name) == 0 {
		return "", "", false
	}
	return namespace, name, true
}

func ContainerFromLocator(locator string) (namespace, name, container string, matches bool) {
	parts := strings.Split(locator, " ")

	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "ns/"):
			tokens := strings.SplitN(part, "/", 2)
			namespace = tokens[1]
		case strings.HasPrefix(part, "pod/"):
			tokens := strings.SplitN(part, "/", 2)
			name = tokens[1]
		case strings.HasPrefix(part, "container/"):
			tokens := strings.SplitN(part, "/", 2)
			container = tokens[1]
		}
	}
	if len(namespace) == 0 || len(name) == 0 || len(container) == 0 {
		return "", "", "", false
	}
	return namespace, name, container, true
}

func LocatePod(pod *corev1.Pod) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s", pod.Namespace, pod.Name, pod.Spec.NodeName)
}

func LocatePodContainer(pod *corev1.Pod, containerName string) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s container/%s", pod.Namespace, pod.Name, pod.Spec.NodeName, containerName)
}
