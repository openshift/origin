package apiserver

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	clihelpers "github.com/openshift/origin/test/extended/cli"
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

	g.It("API LBs follow /readyz of kube-apiserver and stop sending requests before server shutdowns for external clients", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// set up
		// apiserverName -> reader
		logReaders := map[string]io.Reader{}
		results := map[string]int{}

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			mgmtClusterOC := exutil.NewHypershiftManagementCLI("default").AsAdmin().WithoutNamespace()
			pods, err := mgmtClusterOC.KubeClient().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "hypershift.openshift.io/control-plane-component=kube-apiserver"})
			o.Expect(err).To(o.BeNil())
			for _, pod := range pods.Items {
				fileName, err := mgmtClusterOC.Run("logs", "-n", pod.Namespace, pod.Name, "-c", "audit-logs").OutputToFile(pod.Name + "-audit.log")
				o.Expect(err).NotTo(o.HaveOccurred())
				reader, err := os.Open(fileName)
				o.Expect(err).NotTo(o.HaveOccurred())
				defer reader.Close()
				logReaders[pod.Namespace+"-"+pod.Name] = reader
			}
		} else {
			tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
			o.Expect(err).NotTo(o.HaveOccurred())
			defer os.RemoveAll(tempDir)
			args := []string{"--dest-dir", tempDir, "--", "/usr/bin/gather_audit_logs"}

			// download the audit logs from the cluster
			o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())

			// act
			expectedDirectoriesToExpectedCount := []string{path.Join(clihelpers.GetPluginOutputDir(tempDir), "audit_logs", "kube-apiserver")}
			for _, auditDirectory := range expectedDirectoriesToExpectedCount {
				err := filepath.Walk(auditDirectory, func(path string, info os.FileInfo, err error) error {
					g.By(path)
					o.Expect(err).NotTo(o.HaveOccurred())

					if info.IsDir() {
						return nil
					}
					fileName := filepath.Base(path)
					if !clihelpers.IsAuditFile(fileName) {
						return nil
					}

					file, err := os.Open(path)
					o.Expect(err).NotTo(o.HaveOccurred())
					defer file.Close()
					fi, err := file.Stat()
					o.Expect(err).NotTo(o.HaveOccurred())
					if fi.Size() == 0 {
						return nil
					}

					gzipReader, err := gzip.NewReader(file)
					o.Expect(err).NotTo(o.HaveOccurred())

					apiServerName := extractAPIServerNameFromAuditFile(fileName)
					o.Expect(apiServerName).ToNot(o.BeEmpty())

					logReaders[apiServerName] = gzipReader

					return nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		for apiServerName, reader := range logReaders {
			lateRequestCounter := 0
			readFile := false

			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				text := scanner.Text()
				if !strings.HasSuffix(text, "}") {
					continue // ignore truncated data
				}
				o.Expect(text).To(o.HavePrefix(`{"kind":"Event",`))

				if strings.Contains(text, "openshift.io/during-graceful") && strings.Contains(text, "openshift-origin-external-backend-sampler") {
					lateRequestCounter++
				}
				readFile = true
			}
			o.Expect(readFile).To(o.BeTrue())

			if lateRequestCounter > 0 {
				previousLateRequestCounter, _ := results[apiServerName]
				results[apiServerName] = previousLateRequestCounter + lateRequestCounter
			}
		}

		var finalMessageBuilder strings.Builder
		for apiServerName, lateRequestCounter := range results {
			// tolerate a few late request
			if lateRequestCounter > 10 {
				finalMessageBuilder.WriteString(fmt.Sprintf("\n %v observed %v late requests", apiServerName, lateRequestCounter))
			}
		}
		// for now, we will report it as flaky, change it to fail once it proves itself
		if len(finalMessageBuilder.String()) > 0 {
			result.Flakef("the following API Servers observed late requests: %v", finalMessageBuilder.String())
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
