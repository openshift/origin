package legacykubeapiservermonitortests

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testCertRotationFailed(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-kube-apiserver] certificate rotation failed"

	failures := []string{}
	for _, event := range events {
		if event.Message.Reason == "CertificateUpdateFailed" {
			failures = append(failures, fmt.Sprintf("%v %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}
	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	return []*junitapi.JUnitTestCase{{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Certificate rotation failed\n\n%v", strings.Join(failures, "\n")),
		},
	}}
}
