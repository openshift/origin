package admupgradestatus

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var (
	healthLinePattern   = regexp.MustCompile(`^\S+\s+\S+\S+\s+\S+.*$`)
	healthMessageFields = map[string]*regexp.Regexp{
		"Message":            regexp.MustCompile(`^Message:\s+\S+.*$`),
		"Since":              regexp.MustCompile(`^ {2}Since:\s+\S+.*$`),
		"Level":              regexp.MustCompile(`^ {2}Level:\s+\S+.*$`),
		"Impact":             regexp.MustCompile(`^ {2}Impact:\s+\S+.*$`),
		"Reference":          regexp.MustCompile(`^ {2}Reference:\s+\S+.*$`),
		"Resources":          regexp.MustCompile(`^ {2}Resources:$`),
		"resource reference": regexp.MustCompile(`^ {4}[a-z0-9_.-]+: \S+$`),
		"Description":        regexp.MustCompile(`^ {2}Description:\s+\S+.*$`),
	}
)

func (w *monitor) health() *junitapi.JUnitTestCase {
	health := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
		SkipMessage: &junitapi.SkipMessage{
			Message: "Test skipped because no oc adm upgrade status output was successfully collected",
		},
	}

	failureOutputBuilder := strings.Builder{}

	for _, observed := range w.ocAdmUpgradeStatusOutputModels {
		if observed.output == nil {
			// Failing to parse the output is handled in expectedLayout, so we can skip here
			continue
		}
		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		health.SkipMessage = nil

		wroteOnce := false
		fail := func(message string) {
			if !wroteOnce {
				wroteOnce = true
				failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
				failureOutputBuilder.WriteString(observed.output.rawOutput)
				failureOutputBuilder.WriteString(fmt.Sprintf("=> %s\n", message))
			}
		}

		if !observed.output.updating {
			// If the cluster is not updating, workers should not be updating
			if observed.output.health != nil {
				fail("Cluster is not updating but health section is present")
			}
			continue
		}

		h := observed.output.health
		if h == nil {
			fail("Cluster is updating but health section is not present")
			continue
		}

		for _, item := range h.Messages {
			if h.Detailed {
				for field, pattern := range healthMessageFields {
					if !pattern.MatchString(item) {
						fail(fmt.Sprintf("Health message does not contain field %s: %s", field, item))
					}
				}
			} else {
				if !healthLinePattern.MatchString(item) {
					fail(fmt.Sprintf("Health message does not match expected pattern: %s", item))
				}
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		health.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status health section"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return health
}
