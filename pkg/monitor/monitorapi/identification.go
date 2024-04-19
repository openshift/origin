package monitorapi

import (
	"reflect"
	"strings"
)

type BackendConnectionType string

const (
	NewConnectionType    BackendConnectionType = "new"
	ReusedConnectionType BackendConnectionType = "reused"
)

func IsE2ETest(l Locator) bool {
	_, ret := E2ETestFromLocator(l)
	return ret
}

func E2ETestFromLocator(l Locator) (string, bool) {
	test, ok := l.Keys[LocatorE2ETestKey]
	return test, ok
}

func IsNode(locator Locator) bool {
	_, ret := locator.Keys[LocatorNodeKey]
	return ret
}

func NodeFromLocator(locator string) (string, bool) {
	ret := NodeFrom(LocatorParts(locator))
	return ret, len(ret) > 0
}

func NamespaceFromLocator(locator Locator) string {
	return locator.Keys[LocatorNamespaceKey]
}

// TODO: remove all uses
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

func NodeFrom(locatorParts map[string]string) string {
	return locatorParts["node"]
}

func OperatorFrom(locatorParts map[string]string) string {
	return locatorParts[string(LocatorClusterOperatorKey)]
}

func AlertFrom(locatorParts map[string]string) string {
	return locatorParts["alert"]
}

func ThisDisruptionInstanceFrom(locatorParts map[string]string) string {
	return locatorParts[string(LocatorBackendDisruptionNameKey)]
}

func BackendDisruptionNameFromLocator(locator Locator) string {
	return locator.Keys[LocatorBackendDisruptionNameKey]
}

func DisruptionConnectionTypeFrom(locatorParts map[string]string) BackendConnectionType {
	return BackendConnectionType(locatorParts[string(LocatorConnectionKey)])
}

func IsEventForLocator(locator Locator) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return reflect.DeepEqual(eventInterval.Locator, locator)
	}
}

func IsEventForBackendDisruptionName(backendDisruptionName string) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		if BackendDisruptionNameFromLocator(eventInterval.Locator) == backendDisruptionName {
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
