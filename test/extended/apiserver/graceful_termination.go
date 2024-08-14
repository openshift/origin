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
	"regexp"
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

	g.It("kubelet terminates kube-apiserver gracefully extended", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("oc must-gather doesn't currently support external controlPlaneTopology")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		var finalMessageBuilder strings.Builder
		terminationRegexp := regexp.MustCompile(`Previous pod .* did not terminate gracefully`)
		terminationLogsPerServer := gatherMustGatherFor(oc, tempDir, clihelpers.IsTerminationLog, extractAPIServerNameFromTerminationFile)
		for apiServerName, terminationLogs := range terminationLogsPerServer {
			for _, terminationLog := range terminationLogs {
				func() {
					reader, err := newFileWrapper(terminationLog)
					o.Expect(err).NotTo(o.HaveOccurred())
					defer reader.Close()

					scanner := bufio.NewScanner(reader)
					for scanner.Scan() {
						text := scanner.Text()
						if terminationRegexp.MatchString(text) {
							finalMessageBuilder.WriteString(fmt.Sprintf("\n %v wasn't gracefully terminated, reason: %v", apiServerName, text))
						}
					}
					o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
				}()
			}
		}

		if len(finalMessageBuilder.String()) > 0 {
			result.Flakef("The following API Servers weren't gracefully terminated: %v", finalMessageBuilder.String())
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
		// apiserverName -> filePaths
		auditLogsPerServer := map[string][]string{}
		results := map[string]int{}

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			mgmtClusterOC := exutil.NewHypershiftManagementCLI("default").AsAdmin().WithoutNamespace()
			pods, err := mgmtClusterOC.KubeClient().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "hypershift.openshift.io/control-plane-component=kube-apiserver"})
			o.Expect(err).To(o.BeNil())
			for _, pod := range pods.Items {
				filePath, err := mgmtClusterOC.Run("logs", "-n", pod.Namespace, pod.Name, "-c", "audit-logs").OutputToFile(pod.Name + "-audit.log")
				o.Expect(err).NotTo(o.HaveOccurred())
				reader, err := os.Open(filePath)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = reader.Close()
				o.Expect(err).NotTo(o.HaveOccurred())
				auditLogsPerServer[pod.Namespace+"-"+pod.Name] = append(auditLogsPerServer[pod.Namespace+"-"+pod.Name], filePath)
			}
		} else {
			tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
			o.Expect(err).NotTo(o.HaveOccurred())
			defer os.RemoveAll(tempDir)

			auditLogsPerServer = gatherMustGatherFor(oc, tempDir, clihelpers.IsAuditFile, extractAPIServerNameFromAuditFile)
		}

		for apiServerName, auditLogs := range auditLogsPerServer {
			for _, auditLog := range auditLogs {
				lateRequestCounter := 0
				func() {
					reader, err := newFileWrapper(auditLog)
					o.Expect(err).NotTo(o.HaveOccurred())
					defer reader.Close()

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
					}
					o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
				}()

				if lateRequestCounter > 0 {
					previousLateRequestCounter, _ := results[apiServerName]
					results[apiServerName] = previousLateRequestCounter + lateRequestCounter
				}
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

func isGzipFileByExtension(fileName string) bool {
	return strings.HasSuffix(fileName, ".gz")
}

func extractAPIServerNameFromAuditFile(auditFileName string) string {
	pos := strings.Index(auditFileName, "-audit")
	if pos == -1 {
		return ""
	}
	return auditFileName[0:pos]
}

func extractAPIServerNameFromTerminationFile(auditFileName string) string {
	pos := strings.Index(auditFileName, "-termination")
	if pos == -1 {
		return ""
	}
	return auditFileName[0:pos]
}

func gatherMustGatherFor(oc *exutil.CLI, destinationDir string, fileMatcherFn func(string) bool, apiServerNameFromFileExtractorFn func(string) string) map[string][]string {
	args := []string{"--dest-dir", destinationDir, "--", "/usr/bin/gather_audit_logs"}
	o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())

	ret := map[string][]string{}
	for _, auditDirectory := range []string{path.Join(clihelpers.GetPluginOutputDir(destinationDir), "audit_logs", "kube-apiserver")} {
		err := filepath.Walk(auditDirectory, func(path string, info os.FileInfo, err error) error {
			g.By(path)
			o.Expect(err).NotTo(o.HaveOccurred())

			if info.IsDir() {
				return nil
			}
			fileName := filepath.Base(path)
			if !fileMatcherFn(fileName) {
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
			err = gzipReader.Close()
			o.Expect(err).NotTo(o.HaveOccurred())

			apiServerName := apiServerNameFromFileExtractorFn(fileName)
			o.Expect(apiServerName).ToNot(o.BeEmpty())

			ret[apiServerName] = append(ret[apiServerName], path)
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return ret
}

type fileWrapper struct {
	file   *os.File
	reader io.ReadCloser
}

func newFileWrapper(filePath string) (*fileWrapper, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	var reader io.ReadCloser = file
	if isGzipFileByExtension(file.Name()) {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			file.Close() // ignore err
			return nil, err
		}
		reader = gzipReader
	}

	return &fileWrapper{
		file:   file,
		reader: reader,
	}, nil
}

func (fw *fileWrapper) Read(p []byte) (int, error) {
	return fw.reader.Read(p)
}

func (fw *fileWrapper) Close() error {
	if err := fw.reader.Close(); err != nil {
		return err
	}

	// only close the file if it's not the same as the reader
	if fw.reader != fw.file {
		return fw.file.Close()
	}
	return nil
}
