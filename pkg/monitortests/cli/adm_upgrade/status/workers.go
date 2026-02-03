package admupgradestatus

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

var (
	nodeLinePattern = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+\S+.*$`)

	emptyPoolLinePattern = regexp.MustCompile(`^\S+\s+Empty\s+0 Total$`)
	poolLinePattern      = regexp.MustCompile(`^\S+\s+\S+\s+\d+% \(\d+/\d+\)\s+.*$`)
)

func (w *monitor) workers() *junitapi.JUnitTestCase {
	workers := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status workers section is consistent",
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
		workers.SkipMessage = nil

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
			// If the cluster is not updating, workers should not be updating
			if observed.output.workers != nil {
				fail("Cluster is not updating but workers section is present")
			}
			continue
		}

		ws := observed.output.workers
		if ws == nil {
			// We do not show workers in SNO / compact clusters
			// TODO: Crosscheck with topology
			continue
		}

		for _, pool := range ws.Pools {
			if emptyPoolLinePattern.MatchString(pool) {
				name := strings.Split(pool, " ")[0]
				_, ok := ws.Nodes[name]
				if ok {
					fail(fmt.Sprintf("Nodes table should not be shown for an empty pool %s", name))
				}
				continue
			}
			if !poolLinePattern.MatchString(pool) {
				fail(fmt.Sprintf("Bad line in Worker Pool table: %s", pool))
			}
		}

		if len(ws.Nodes) > len(ws.Pools) {
			fail("Showing more Worker Pool Nodes tables than lines in Worker Pool table")
		}

		for name, nodes := range ws.Nodes {
			if len(nodes) == 0 {
				fail(fmt.Sprintf("Worker Pool Nodes table for %s is empty", name))
				continue
			}

			for _, node := range nodes {
				if !nodeLinePattern.MatchString(node) {
					fail(fmt.Sprintf("Bad line in Worker Pool Nodes table for %s: %s", name, node))
				}
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		workers.FailureOutput = &junitapi.FailureOutput{
			Output: failureOutputBuilder.String(),
		}
	}

	return workers
}
