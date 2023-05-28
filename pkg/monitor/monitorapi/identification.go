package monitorapi

import (
	"fmt"
	"strconv"
	"strings"
)

type BackendConnectionType string

const (
	NewConnectionType    BackendConnectionType = "new"
	ReusedConnectionType BackendConnectionType = "reused"
)

func LocateRouteForDisruptionCheck(ns, name, disruptionBackendName string, connectionType BackendConnectionType) string {
	return fmt.Sprintf("ns/%s route/%s disruption/%s connection/%s", ns, name, disruptionBackendName, connectionType)
}

func LocateDisruptionCheck(disruptionBackendName string, connectionType BackendConnectionType) string {
	return fmt.Sprintf("disruption/%s connection/%s", disruptionBackendName, connectionType)
}

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
	parts := strings.SplitN(strings.TrimPrefix(locator, "node/"), " ", 2)
	return parts[0], true
}

func OperatorLocator(operatorName string) string {
	return fmt.Sprintf("clusteroperator/%v", operatorName)
}

func OperatorFromLocator(locator string) (string, bool) {
	if !strings.HasPrefix(locator, "clusteroperator/") {
		return "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(locator, "clusteroperator/"), " ", 2)
	return parts[0], true
}

func LocatorParts(locator string) map[string]string {
	parts := map[string]string{}

	tags := strings.Split(locator, " ")
	for _, tag := range tags {
		keyValue := strings.SplitN(tag, "/", 2)
		if len(keyValue) == 1 {
			parts[keyValue[0]] = ""
		} else {
			parts[keyValue[0]] = keyValue[1]
		}
	}

	return parts
}

func NamespaceFrom(locatorParts map[string]string) string {
	if ns, ok := locatorParts["ns"]; ok {
		return ns
	}
	if ns, ok := locatorParts["namespace"]; ok {
		return ns
	}
	return ""
}

func NamespaceFromLocator(locator string) string {
	locatorParts := LocatorParts(locator)
	if ns, ok := locatorParts["ns"]; ok {
		return ns
	}
	if ns, ok := locatorParts["namespace"]; ok {
		return ns
	}
	return ""
}

func AlertFromLocator(locator string) string {
	return AlertFrom(LocatorParts(locator))
}

func AlertFrom(locatorParts map[string]string) string {
	return locatorParts["alert"]
}

func DisruptionFrom(locatorParts map[string]string) string {
	return locatorParts["disruption"]
}

func DisruptionConnectionTypeFrom(locatorParts map[string]string) string {
	return locatorParts["connection"]
}

func DisruptionLoadBalancerTypeFrom(locatorParts map[string]string) string {
	return locatorParts["load-balancer"]
}

func DisruptionProtocolFrom(locatorParts map[string]string) string {
	return locatorParts["protocol"]
}

func DisruptionTargetAPIFrom(locatorParts map[string]string) string {
	return locatorParts["target"]
}

func IsEventForLocator(locator string) EventIntervalMatchesFunc {
	return func(eventInterval EventInterval) bool {
		if eventInterval.Locator == locator {
			return true
		}
		return false
	}
}

type NamespacedReference struct {
	Namespace string
	Name      string
	UID       string
}
