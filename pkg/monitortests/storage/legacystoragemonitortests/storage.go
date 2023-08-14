package legacystoragemonitortests

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type throttlingMessage struct {
	cloud   string
	message *regexp.Regexp
}

var throttlingMessages = []*regexp.Regexp{
	// GCE
	regexp.MustCompile("googleapi: Error 403: Quota exceeded"),
	// AWS
	regexp.MustCompile("RequestLimitExceeded"),
}

func testAPIQuotaEvents(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] cloud API quota should not be exceeded"

	var matches []string
	for i := range events {
		event := events[i]
		for _, msg := range throttlingMessages {
			if msg.MatchString(event.Message) {
				matches = append(matches, event.Message)
			}
		}
	}

	if len(matches) > 0 {
		output := fmt.Sprintf("Underlying cloud was rate limiting OCP's API calls:\n%s", strings.Join(matches, "\n"))
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: output,
				},
			},
			// Mark the test as a flake
			{
				Name: testName,
			},
		}
	}

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
		},
	}
}
