package apiserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("terminating-kube-apiserver")

	// This test checks whether the apiserver reports any events that may indicate a problem at any time,
	// not just when the suite is running. We already have invariant tests that fail if these are violated
	// during suite execution, but we want to know if there are fingerprints of these failures outside of tests.
	g.It("kubelet terminates kube-apiserver gracefully", func() {
		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		var messages []string
		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "NonGracefulTermination" {
				continue
			}
			data, _ := json.Marshal(ev)
			messages = append(messages, string(data))
		}
		if len(messages) > 0 {
			result.Flakef("kube-apiserver reported a non-graceful termination (after %s which is test environment dependent). Probably kubelet or CRI-O is not giving the time to cleanly shut down. This can lead to connection refused and network I/O timeout errors in other components.\n\n%s", eventsAfterTime, strings.Join(messages, "\n"))
		}
	})

	// This test extends the previous test by checking the content of the termination files for kube-apiservers.
	// It should catch cases where the event is not persisted in the database. It should also catch
	// cases where the KAS is immediately restarted or shut down after an ungraceful termination.
	g.It("kubelet terminates kube-apiserver gracefully extended", func() {
		var finalMessageBuilder strings.Builder
		terminationRegexp := regexp.MustCompile(`Previous pod .* did not terminate gracefully`)
		// klog timestamp format: W0120 22:20:50.473381
		klogTimestampRegexp := regexp.MustCompile(`^[IWEF](\d{4}) (\d{2}:\d{2}:\d{2}\.\d+)`)

		masters, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, master := range masters.Items {
			g.By(fmt.Sprintf("Getting log files for kube-apiserver on master: %s", master.Name))
			kasLogFileNames, _, err := oc.AsAdmin().Run("adm").Args("node-logs", master.Name, "--path=kube-apiserver/").Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, kasLogFileName := range strings.Split(kasLogFileNames, "\n") {
				if !isKASTerminationLogFile(kasLogFileName) {
					continue
				}
				g.By(fmt.Sprintf("Getting and processing %s file for kube-apiserver on master: %s", kasLogFileName, master.Name))
				kasTerminationFileOutput, _, err := oc.AsAdmin().Run("adm").Args("node-logs", master.Name, fmt.Sprintf("--path=kube-apiserver/%s", kasLogFileName)).Outputs()
				o.Expect(err).NotTo(o.HaveOccurred())
				kasTerminationFileReader := strings.NewReader(kasTerminationFileOutput)
				kasTerminationFileScanner := bufio.NewScanner(kasTerminationFileReader)
				for kasTerminationFileScanner.Scan() {
					line := kasTerminationFileScanner.Text()
					if terminationRegexp.MatchString(line) {
						observedAt := parseKlogTimestamp(line, klogTimestampRegexp)
						finalMessageBuilder.WriteString(fmt.Sprintf("\n kube-apiserver on node %s wasn't gracefully terminated (observed at %s), reason: %s", master.Name, observedAt, line))
					}
				}
				o.Expect(kasTerminationFileScanner.Err()).NotTo(o.HaveOccurred())
			}
		}
		if len(finalMessageBuilder.String()) > 0 {
			g.GinkgoT().Errorf("The following API Servers weren't gracefully terminated: %v", finalMessageBuilder.String())
		}
	})

	// This test checks whether the apiserver reports any events that may indicate a problem at any time,
	// not just when the suite is running. We already have invariant tests that fail if these are violated
	// during suite execution, but we want to know if there are fingerprints of these failures outside of tests.
	g.It("kube-apiserver terminates within graceful termination period", func() {
		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		var messages []string
		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "GracefulTerminationTimeout" {
				continue
			}
			data, _ := json.Marshal(ev)
			messages = append(messages, string(data))
		}
		if len(messages) > 0 {
			result.Flakef("kube-apiserver didn't terminate by itself during the graceful termination period (after %s which is test environment dependent). This is a bug in kube-apiserver. It probably means that network connections are not closed cleanly, and this leads to network I/O timeout errors in other components.\n\n%s", eventsAfterTime, strings.Join(messages, "\n"))
		}
	})

	// This test is outcome-based: a LateConnections event only proves that a load balancer
	// (cloud LB, on-prem haproxy/keepalived, or the in-cluster kubernetes.default service path)
	// routed a *new* connection to a terminating kube-apiserver late in its shutdown window.
	// That alone is not client-visible breakage: with shutdown-send-retry-after enabled (the
	// OpenShift default), such requests are rejected with 429 + Retry-After + Connection: close
	// and well-behaved clients transparently retry against a healthy backend. No LB
	// implementation can be perfectly synchronized with /readyz, so slow LB convergence is
	// expected occasionally (see OCPBUGS-86789).
	//
	// Therefore:
	//   - late requests that were gracefully rejected (429) are tolerated but reported as a
	//     flake, keeping a fingerprint of slow LB convergence without failing the run,
	//   - late requests that were processed or failed hard (5xx) fail the test, because that
	//     means the graceful termination machinery did not protect the client.
	g.It("API LBs follow /readyz of kube-apiserver and stop sending requests", func() {
		t := g.GinkgoT()

		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		type lateWindow struct {
			node  string
			from  time.Time
			to    time.Time
			event corev1.Event
		}
		var windows []lateWindow
		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "LateConnections" {
				continue
			}
			node := nodeNameFromKASEvent(&ev)
			if node == "" {
				// cannot attribute the event to a node; keep the old, conservative behaviour
				t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver far too late in termination process, probably due to broken LB configuration: %#v. This can lead to connection refused and network I/O timeout errors in other components.", ev)
				continue
			}
			windows = append(windows, lateWindow{
				node: node,
				// The event is emitted when the terminating listener observes late
				// connections, which happens during the last 20% of the
				// shutdown-delay-duration (at most ~39s for the largest configured
				// value of 194s). Bracket conservatively around the event.
				from:  ev.LastTimestamp.Time.Add(-60 * time.Second),
				to:    ev.LastTimestamp.Time.Add(10 * time.Second),
				event: ev,
			})
		}
		if len(windows) == 0 {
			return
		}

		var hardFailures, gracefulRejections, unattributed []string
		for _, w := range windows {
			entries, err := auditEntriesForNodeInWindow(oc, w.node, w.from, w.to)
			if err != nil {
				// without audit data we cannot prove the outcome was safe; fail loud
				t.Errorf("API LBs routed late connections to terminating kube-apiserver on node %s (event: %#v) and audit logs could not be inspected to verify the outcome: %v", w.node, w.event, err)
				continue
			}

			late429s, late5xxs := 0, 0
			for _, e := range entries {
				if e.ResponseStatus == nil {
					continue
				}
				switch {
				case e.ResponseStatus.Code == http.StatusTooManyRequests:
					late429s++
					gracefulRejections = append(gracefulRejections,
						fmt.Sprintf("node=%s verb=%s uri=%q ua=%q src=%v -> 429 (gracefully rejected)",
							w.node, e.Verb, e.RequestURI, e.UserAgent, e.SourceIPs))
				case e.ResponseStatus.Code >= 500:
					late5xxs++
					hardFailures = append(hardFailures,
						fmt.Sprintf("node=%s verb=%s uri=%q ua=%q src=%v -> %d",
							w.node, e.Verb, e.RequestURI, e.UserAgent, e.SourceIPs, e.ResponseStatus.Code))
				}
				// 2xx responses in the window are expected: they belong to in-flight
				// requests that started before the drain and are being finished
				// gracefully. Only new (late) connections matter, and those are exactly
				// the ones the shutdown-send-retry-after filter turns into 429s.
			}
			if late429s == 0 && late5xxs == 0 {
				// The listener saw late connections but no request on them was ever
				// answered — the connections were likely reset without a response,
				// which is the worst outcome for clients.
				unattributed = append(unattributed,
					fmt.Sprintf("node=%s event=%s", w.node, w.event.Message))
			}
		}

		if len(hardFailures) > 0 {
			t.Errorf("late requests reached a terminating kube-apiserver and failed hard (the LB routed late AND graceful rejection did not protect the client). This can lead to connection refused and network I/O timeout errors in other components:\n%s", strings.Join(hardFailures, "\n"))
		}
		if len(unattributed) > 0 {
			t.Errorf("API LBs routed new connections to a terminating kube-apiserver late in the termination process and no graceful rejection was recorded in the audit log; the connections were probably reset, probably due to broken LB configuration. This can lead to connection refused and network I/O timeout errors in other components:\n%s", strings.Join(unattributed, "\n"))
		}
		if len(hardFailures) == 0 && len(unattributed) == 0 && len(gracefulRejections) > 0 {
			result.Flakef("API LBs sent new connections to kube-apiserver late in the termination process, but all of them were gracefully rejected with 429 + Retry-After (no client impact). This is a fingerprint of slow LB convergence relative to /readyz:\n%s", strings.Join(gracefulRejections, "\n"))
		}
	})

	g.It("API LBs follow /readyz of kube-apiserver and don't send request early", ote.Informing(), func() {
		t := g.GinkgoT()

		client, err := kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		evs, err := client.CoreV1().Events("openshift-kube-apiserver").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "NonReadyRequests" {
				continue
			}

			t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver before it is ready, probably due to broken LB configuration: %#v. This can lead to inconsistent responses like 403s in other components.", ev)
		}
	})
})

func extractAPIServerNameFromAuditFile(auditFileName string) string {
	pos := strings.Index(auditFileName, "-audit")
	if pos == -1 {
		return ""
	}
	return auditFileName[0:pos]
}

// nodeNameFromKASEvent derives the node a kube-apiserver event was emitted from.
// Events emitted by the kube-apiserver reference the static pod
// "kube-apiserver-<node>" as the involved object; the source host is used as a
// fallback when set.
func nodeNameFromKASEvent(ev *corev1.Event) string {
	if name := ev.InvolvedObject.Name; strings.HasPrefix(name, "kube-apiserver-") {
		return strings.TrimPrefix(name, "kube-apiserver-")
	}
	return ev.Source.Host
}

// auditEntriesForNodeInWindow fetches kube-apiserver audit log entries from a
// control plane node, restricted to requests received within [from, to].
// Audit files rotate, so all kube-apiserver audit files on the node are
// scanned and filtered by RequestReceivedTimestamp.
func auditEntriesForNodeInWindow(oc *exutil.CLI, node string, from, to time.Time) ([]auditv1.Event, error) {
	fileNames, _, err := oc.AsAdmin().Run("adm").Args("node-logs", node, "--path=kube-apiserver/").Outputs()
	if err != nil {
		return nil, fmt.Errorf("listing kube-apiserver log files on node %s: %w", node, err)
	}
	var out []auditv1.Event
	for _, name := range strings.Split(fileNames, "\n") {
		name = strings.TrimSpace(name)
		if !strings.Contains(name, "audit") || !strings.HasSuffix(name, ".log") {
			continue
		}
		raw, _, err := oc.AsAdmin().Run("adm").Args("node-logs", node, fmt.Sprintf("--path=kube-apiserver/%s", name)).Outputs()
		if err != nil {
			// the file may have been rotated away between listing and reading
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(raw))
		scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var e auditv1.Event
			if err := json.Unmarshal(line, &e); err != nil {
				continue
			}
			ts := e.RequestReceivedTimestamp.Time
			if ts.Before(from) || ts.After(to) {
				continue
			}
			out = append(out, e)
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanning audit file %s on node %s: %w", name, node, err)
		}
	}
	return out, nil
}

func isKASTerminationLogFile(fileName string) bool {
	return strings.Contains(fileName, "termination")
}

// parseKlogTimestamp extracts and formats the klog timestamp from a log line.
// klog timestamps have the format: W0120 22:20:50.473381 (MMDD HH:MM:SS.ffffff without year).
// We use the current year as klog does not include it.
func parseKlogTimestamp(line string, klogTimestampRegexp *regexp.Regexp) string {
	matches := klogTimestampRegexp.FindStringSubmatch(line)
	if len(matches) < 3 {
		return "unknown"
	}
	// matches[1] = "0120" (MMDD), matches[2] = "22:20:50.473381"
	timestampStr := fmt.Sprintf("%d-%s-%s", time.Now().Year(), matches[1][:2]+"-"+matches[1][2:], matches[2])
	t, err := time.Parse("2006-01-02-15:04:05.000000", timestampStr)
	if err != nil {
		return matches[1] + " " + matches[2]
	}
	return t.Format(time.RFC3339)
}
