package apiserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		eventsAfterTime := exutil.LimitTestsToStartTime()
		for _, ev := range evs.Items {
			if ev.LastTimestamp.Time.Before(eventsAfterTime) {
				continue
			}
			if ev.Reason != "LateConnections" {
				continue
			}

			t.Errorf("API LBs or the kubernetes service send requests to kube-apiserver far too late in termination process, probably due to broken LB configuration: %#v. This can lead to connection refused and network I/O timeout errors in other components.", ev)
		}
	})

	g.It("API LBs follow /readyz of kube-apiserver and don't send request early", func() {
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
