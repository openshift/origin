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
		"Message":            regexp.MustCompile(`(?m)^Message: +\S+.*$`),
		"Since":              regexp.MustCompile(`(?m)^ {2}Since: +\S+.*$`),
		"Level":              regexp.MustCompile(`(?m)^ {2}Level: +\S+.*$`),
		"Impact":             regexp.MustCompile(`(?m)^ {2}Impact: +\S+.*$`),
		"Reference":          regexp.MustCompile(`(?m)^ {2}Reference: +\S+.*$`),
		"Resources":          regexp.MustCompile(`(?m)^ {2}Resources:$`),
		"resource reference": regexp.MustCompile(`(?m)^ {4}[a-z0-9_.-]+: +\S+$`),
		"Description":        regexp.MustCompile(`(?m)^ {2}Description: +\S+.*$`),
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
				failureOutputBuilder.WriteString(fmt.Sprintf("\n\n=> %s\n", message))
			}
		}

		if !observed.output.updating {
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
					if pattern.FindString(item) == "" {
						fail(fmt.Sprintf("Health message does not contain field %s", field))
					}
				}
			} else {
				if !healthLinePattern.MatchString(item) {
					fail(fmt.Sprintf("Health message does not match expected pattern:\n%s", item))
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
