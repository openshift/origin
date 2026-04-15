package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
)

// goroutineHeaderRe matches Go goroutine dump headers, e.g.:
//
//	goroutine 64418 [IO wait, 189 minutes]:
//	goroutine 1 [running]:
var goroutineHeaderRe = regexp.MustCompile(`goroutine \d+ \[`)

// ioWaitDurationRe extracts the wait duration in minutes from an IO wait goroutine header, e.g.:
//
//	goroutine 64418 [IO wait, 189 minutes]:
var ioWaitDurationRe = regexp.MustCompile(`goroutine \d+ \[IO wait, (\d+) minutes\]:`)

// findStuckImagePulls checks a goroutine dump for goroutines that indicate
// a stuck image pull: IO wait > 30 minutes with net.(*conn).Read called by
// docker.(*bodyReader).Read.
func findStuckImagePulls(dump string) []string {
	// Split the dump into individual goroutine blocks.
	// Each block starts with "goroutine <id> [".
	blocks := goroutineHeaderRe.Split(dump, -1)
	headers := goroutineHeaderRe.FindAllString(dump, -1)

	var stuck []string
	for i, header := range headers {
		block := header + blocks[i+1]

		// Check: state is "IO wait" with duration > 30 minutes
		match := ioWaitDurationRe.FindStringSubmatch(block)
		if match == nil {
			continue
		}
		minutes, err := strconv.Atoi(match[1])
		if err != nil || minutes <= 30 {
			continue
		}

		// Check: stack contains net.(*conn).Read
		connReadIdx := strings.Index(block, "net.(*conn).Read")
		if connReadIdx < 0 {
			continue
		}

		// Check: docker.(*bodyReader).Read appears as an ascendant
		// (caller) of net.(*conn).Read. In a goroutine dump callers
		// appear below callees, so bodyReader must come after
		// conn.Read in the text. The match is version-agnostic so
		// it keeps working across containers/image version bumps.
		bodyReaderIdx := strings.Index(block, "docker.(*bodyReader).Read")
		if bodyReaderIdx < 0 || bodyReaderIdx <= connReadIdx {
			continue
		}

		stuck = append(stuck, strings.TrimSpace(block))
	}
	return stuck
}

var _ = g.Describe("[sig-node][Late]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("crio-goroutine-dump")

	g.It("CRI-O should report goroutine stacks on all nodes",
		ote.Informing(), func(ctx g.SpecContext) {

			nodes, err := exutil.GetAllClusterNodes(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nodes).NotTo(o.BeEmpty(), "expected at least one node")

			// Send SIGUSR1 to CRI-O and read the newly created dump file.
			// CRI-O writes goroutine stacks to /tmp/crio-goroutine-stacks-<timestamp>.log.
			//
			// We access CRI-O's /tmp through /proc/<pid>/root/tmp rather than
			// the host /tmp so that the dump is visible even when CRI-O's
			// systemd unit uses PrivateTmp=yes (as observed on MicroShift).
			//
			// The script snapshots the latest dump file before signalling and
			// polls until a newer, non-empty (-s) file appears to avoid
			// reading a file that CRI-O has created but not yet finished writing.
			shellCmd := `CRIO_PID=$(pgrep -x crio 2>/dev/null)
if [ -z "$CRIO_PID" ]; then echo "CRIO_NOT_FOUND"; exit 0; fi
CRIO_TMP=/proc/$CRIO_PID/root/tmp
BEFORE=$(ls -t "$CRIO_TMP"/crio-goroutine-stacks-*.log 2>/dev/null | head -1)
kill -USR1 $CRIO_PID
for i in $(seq 1 30); do
  LATEST=$(ls -t "$CRIO_TMP"/crio-goroutine-stacks-*.log 2>/dev/null | head -1)
  if [ -n "$LATEST" ] && [ "$LATEST" != "$BEFORE" ] && [ -s "$LATEST" ]; then
    cat "$LATEST"; exit 0
  fi
  sleep 1
done
echo "DUMP_TIMEOUT"; exit 1`

			// nodeResult holds the output from a single node's goroutine dump.
			type nodeResult struct {
				name   string
				output string
				err    error
			}

			// Create debug pods serially to avoid putting resource pressure on the API server.
			results := make([]nodeResult, len(nodes))
			for i, node := range nodes {
				g.By(fmt.Sprintf("Sending SIGUSR1 to CRI-O on node %s", node.Name))

				output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
					oc, node.Name, "default",
					"sh", "-c", shellCmd,
				)
				results[i] = nodeResult{name: node.Name, output: output, err: err}
			}

			var stuckPulls []string
			var failedNodes []string
			for _, r := range results {
				// Check output-based diagnostics before the generic error
				// because DebugNodeRetryWithOptionsAndChroot may return
				// both output and an error (e.g. non-zero exit from the
				// DUMP_TIMEOUT branch).
				if strings.Contains(r.output, "CRIO_NOT_FOUND") {
					failedNodes = append(failedNodes,
						fmt.Sprintf("%s: CRI-O process not found", r.name))
					continue
				}

				if strings.Contains(r.output, "DUMP_TIMEOUT") {
					failedNodes = append(failedNodes,
						fmt.Sprintf("%s: timed out waiting for new goroutine dump file", r.name))
					continue
				}

				if r.err != nil {
					failedNodes = append(failedNodes,
						fmt.Sprintf("%s: debug pod failed: %v", r.name, r.err))
					continue
				}

				o.Expect(goroutineHeaderRe.MatchString(r.output)).To(o.BeTrue(),
					"expected goroutine stacks in CRI-O dump from node %s, output length=%d, got:\n%s", r.name, len(r.output), r.output)

				for _, goroutine := range findStuckImagePulls(r.output) {
					stuckPulls = append(stuckPulls, fmt.Sprintf("node/%s:\n%s", r.name, goroutine))
				}
			}

			// Fail if any node did not produce a dump
			o.Expect(failedNodes).To(o.BeEmpty(),
				"failed to collect CRI-O goroutine dump from nodes:\n%s",
				strings.Join(failedNodes, "\n"))

			// Fail hard if any goroutine is stuck in an image pull
			o.Expect(stuckPulls).To(o.BeEmpty(),
				"found CRI-O goroutines stuck in image pull (IO wait > 30 min):\n%s",
				strings.Join(stuckPulls, "\n\n"))
		})
})
