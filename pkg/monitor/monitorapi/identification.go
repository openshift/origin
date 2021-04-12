package monitorapi

import (
	"fmt"
	"strconv"
	"strings"
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
