package monitorapi

import (
	"fmt"
	"strconv"
	"strings"
)

func E2ETestLocator(testName, jUnitSuiteName string) string {
	return fmt.Sprintf("e2e-test/%q jUnitSuite/%s", testName, jUnitSuiteName)
}

func IsE2ETest(locator string) bool {
	_, ret := E2ETestFromLocator(locator)
	return ret
}

func E2ETestFromLocator(locator string) (string, bool) {
	locatorParts := LocatorParts(locator)
	if quotedTestName, ok := locatorParts["e2e-test"]; ok {
		testName, err := strconv.Unquote(quotedTestName)
		if err != nil {
			return "", false
		}
		return testName, true
	}
	return "", false
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
	parts := strings.SplitN(strings.TrimPrefix(locator, "clusteroperator/"), " ", 2)
	return parts[0], true
}

func LocatorParts(locator string) map[string]string {
	parts := map[string]string{}

	// sorry but we had to get clever here.
	// we sometimes use locators with: e2e-test/"my multiword test name" foo/bar
	// we use a csv splitter on " " to handle the possibility of quotes within a segment:
	// courtesy of https://stackoverflow.com/questions/47489745/splitting-a-string-at-space-except-inside-quotation-marks
	// Split string
	/*
		r := csv.NewReader(strings.NewReader(locator))
		r.Comma = ' ' // space
		r.LazyQuotes = true
		tags, err := r.Read()
		if err != nil {
			log.WithError(err).Fatalf("error parsing locator: %s", locator)
		}
	*/

	quoted := false
	tags := strings.FieldsFunc(locator, func(r rune) bool {
		if r == '"' {
			quoted = !quoted
		}
		return !quoted && r == ' '
	})

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

func AlertFrom(locatorParts map[string]string) string {
	return locatorParts["alert"]
}

func DisruptionFrom(locatorParts map[string]string) string {
	return locatorParts["disruption"]
}

func DisruptionConnectionTypeFrom(locatorParts map[string]string) string {
	return locatorParts["connection"]
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
