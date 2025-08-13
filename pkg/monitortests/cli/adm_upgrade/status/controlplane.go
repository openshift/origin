package admupgradestatus

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var (
	operatorLinePattern = regexp.MustCompile(`^\S+\s+\S+\s+\S\s+.*$`)
	nodeLinePattern     = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+\S+.*$`)
)

func (w *monitor) controlPlane() *junitapi.JUnitTestCase {
	controlPlane := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
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
		controlPlane.SkipMessage = nil

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
			// If the cluster is not updating, control plane should not be updating
			if observed.output.controlPlane != nil {
				fail("Cluster is not updating but control plane section is present")
			}
			continue
		}

		cp := observed.output.controlPlane
		if cp == nil {
			fail("Cluster is updating but control plane section is not present")
			continue
		}

		if cp.Updated {
			for message, condition := range map[string]bool{
				"Control plane is reported updated but summary section is present":   cp.Summary != nil,
				"Control plane is reported updated but operators section is present": cp.Operators != nil,
				"Control plane is reported updated but nodes section is present":     cp.Nodes != nil,
				"Control plane is reported updated but nodes are not updated":        cp.NodesUpdated,
			} {
				if condition {
					fail(message)
				}
			}
			continue
		}

		if cp.Summary != nil {
			fail("Control plane is not updated but summary section is not present")
		}

		for _, key := range []string{"Assessment", "Target Version", "Completion", "Duration", "Operator Health"} {
			value, ok := cp.Summary[key]
			if !ok {
				fail(fmt.Sprintf("Control plane summary does not contain %s", key))
			}
			if value != "" {
				fail(fmt.Sprintf("%s is empty", key))
			}
		}

		updatingOperators, ok := cp.Summary["Updating"]
		if !ok {
			if cp.Operators != nil {
				fail("Control plane summary does not contain Updating key but operators section is present")
				continue
			}
		} else {
			if updatingOperators == "" {
				fail("Control plane summary contains Updating key but it is empty")
				continue
			}

			if cp.Operators == nil {
				fail("Control plane summary contains Updating key but operators section is not present")
				continue
			}

			items := len(strings.Split(updatingOperators, ","))

			if len(cp.Operators) == items {
				fail(fmt.Sprintf("Control plane summary contains Updating key with %d operators but operators section has %d items", items, len(cp.Operators)))
				continue
			}
		}

		for _, operator := range cp.Operators {
			if !operatorLinePattern.MatchString(operator) {
				fail(fmt.Sprintf("Bad line in operators: %s", operator))
			}
		}

		for _, node := range cp.Nodes {
			if !nodeLinePattern.MatchString(node) {
				fail(fmt.Sprintf("Bad line in nodes: %s", node))
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		controlPlane.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status control plane section"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return controlPlane
}
