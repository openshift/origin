package legacykubeapiservermonitortests

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testAPIServerIPTablesAccessDisruption(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-kube-apiserver] kube-apiserver should be accessible by clients using internal load balancer without iptables issues"
	namespacesToCount := map[string]int{}
	messages := []string{}
	for _, event := range events {
		reason := monitorapi.ReasonFrom(event.Message)
		if reason != "iptables-operation-not-permitted" {
			continue
		}
		ns := monitorapi.NamespaceFromLocator(event.Locator)
		namespacesToCount[ns] = namespacesToCount[ns] + 1
		messages = append(messages, event.String())
	}

	var tests []*junitapi.JUnitTestCase
	successTest := &junitapi.JUnitTestCase{
		Name: testName,
	}
	if len(messages) > 0 {
		failureOutput := ""
		for _, ns := range sets.StringKeySet(namespacesToCount).List() {
			failureOutput += fmt.Sprintf("namespace/%v has %d instances of 'write: operation not permitted'\n", ns, namespacesToCount[ns])
		}
		failureOutput += "\n\n"
		failureOutput += strings.Join(messages, "\n")

		failureTest := &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: failureOutput,
			},
		}
		tests = append(tests, failureTest)
		tests = append(tests, successTest) // ensures we only flake, no fail.  so far.

	} else {
		tests = append(tests, successTest) // ensures we have success when appropriate
	}
	return tests

}
