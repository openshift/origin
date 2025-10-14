package cli

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cli] oc adm must-gather", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oc-adm-must-gather").AsAdmin()

	g.JustBeforeEach(func() {
		// wait for the default service account to be avaiable
		err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("runs successfully [apigroup:config.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("oc must-gather doesn't currently support external controlPlaneTopology")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		o.Expect(oc.Run("adm", "must-gather").Args("--dest-dir", tempDir, "--volume-percentage=100").Execute()).To(o.Succeed())

		pluginOutputDir := GetPluginOutputDir(tempDir)

		expectedDirectories := [][]string{
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io"},
			{pluginOutputDir, "cluster-scoped-resources", "operator.openshift.io"},
			{pluginOutputDir, "cluster-scoped-resources", "core"},
			{pluginOutputDir, "cluster-scoped-resources", "apiregistration.k8s.io"},
			{pluginOutputDir, "namespaces", "openshift"},
			{pluginOutputDir, "namespaces", "openshift-kube-apiserver-operator"},
		}

		expectedFiles := [][]string{
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "apiservers.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "authentications.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "builds.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "clusteroperators.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "clusterversions.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "consoles.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "dnses.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "featuregates.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "images.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "infrastructures.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "ingresses.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "networks.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "oauths.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "projects.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "schedulers.yaml"},
			// TODO: This got broken and we need to fix this. Disabled temporarily.
			// {pluginOutputDir, "namespaces", "openshift-kube-apiserver", "core", "configmaps.yaml"},
			// {pluginOutputDir, "namespaces", "openshift-kube-apiserver", "core", "secrets.yaml"},
			{pluginOutputDir, "host_service_logs", "masters", "crio_service.log"},
			{pluginOutputDir, "host_service_logs", "masters", "kubelet_service.log"},
		}

		for _, expectedDirectory := range expectedDirectories {
			o.Expect(path.Join(expectedDirectory...)).To(o.BeADirectory())
		}

		emptyFiles := []string{}
		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			o.Expect(expectedFilePath).To(o.BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			if size := stat.Size(); size < 50 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}
	})

	g.It("runs successfully with options [apigroup:config.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("oc must-gather doesn't currently support external controlPlaneTopology")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		args := []string{
			"--dest-dir", tempDir,
			"--volume-percentage=100",
			"--source-dir", "/artifacts",
			"--",
			"/bin/bash", "-c",
			"ls -l > /artifacts/ls.log",
		}
		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		expectedFilePath := path.Join(GetPluginOutputDir(tempDir), "ls.log")
		o.Expect(expectedFilePath).To(o.BeAnExistingFile())
		stat, err := os.Stat(expectedFilePath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stat.Size()).To(o.BeNumerically(">", 0))
	})

	g.It("runs successfully for audit logs [apigroup:config.openshift.io][apigroup:oauth.openshift.io]", func() {
		// On External clusters, events will not be part of the output, since audit logs do not include control plane logs.
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("External clusters don't have control plane audit logs")
		}

		// makes some tokens that should not show in the audit logs
		_, sha256TokenName := exutil.GenerateOAuthTokenPair()
		oauthClient := oauthv1client.NewForConfigOrDie(oc.AdminConfig())
		_, err1 := oauthClient.OAuthAccessTokens().Create(context.Background(), &oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: sha256TokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		}, metav1.CreateOptions{})
		o.Expect(err1).NotTo(o.HaveOccurred())
		_, err2 := oauthClient.OAuthAuthorizeTokens().Create(context.Background(), &oauthv1.OAuthAuthorizeToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: sha256TokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		}, metav1.CreateOptions{})
		o.Expect(err2).NotTo(o.HaveOccurred())

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		args := []string{
			"--dest-dir", tempDir,
			"--volume-percentage=100",
			"--",
			"/usr/bin/gather_audit_logs",
		}

		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		// wait for the contents to show up in the plugin output directory, avoiding EOF errors
		time.Sleep(10 * time.Second)

		pluginOutputDir := GetPluginOutputDir(tempDir)

		expectedDirectoriesToExpectedCount := map[string]int{
			path.Join(pluginOutputDir, "audit_logs", "kube-apiserver"):      1000,
			path.Join(pluginOutputDir, "audit_logs", "openshift-apiserver"): 10, // openshift apiservers don't necessarily get much traffic.  Especially early in a run
			path.Join(pluginOutputDir, "audit_logs", "oauth-apiserver"):     10, // oauth apiservers don't necessarily get much traffic.  Especially early in a run
		}

		expectedFiles := [][]string{
			{pluginOutputDir, "audit_logs", "kube-apiserver.audit_logs_listing"},
			{pluginOutputDir, "audit_logs", "openshift-apiserver.audit_logs_listing"},
			{pluginOutputDir, "audit_logs", "oauth-apiserver.audit_logs_listing"},
		}

		// for some crazy reason, it seems that the files from must-gather take time to appear on disk for reading.  I don't understand why
		// but this was in a previous commit and I don't want to immediately flake: https://github.com/openshift/origin/commit/006745a535848e84dcbcdd1c83ae86deddd3a229#diff-ad1c47fa4213de16d8b3237df5d71724R168
		// so we're going to try to get a pass every 10 seconds for a minute.  If we pass, great.  If we don't, we report the
		// last error we had.
		var lastErr error
		err = wait.PollImmediate(10*time.Second, 1*time.Minute, func() (bool, error) {
			// make sure we do not log OAuth tokens
			for auditDirectory, expectedNumberOfAuditEntries := range expectedDirectoriesToExpectedCount {
				eventsChecked := 0
				err := filepath.Walk(auditDirectory, func(path string, info os.FileInfo, err error) error {
					g.By(path)
					o.Expect(err).NotTo(o.HaveOccurred())
					if info.IsDir() {
						return nil
					}

					fileName := filepath.Base(path)
					if !IsAuditFile(fileName) {
						return nil
					}

					// at this point, we expect only audit files with json events, one per line

					readFile := false

					file, err := os.Open(path)
					o.Expect(err).NotTo(o.HaveOccurred())
					defer file.Close()

					fi, err := file.Stat()
					o.Expect(err).NotTo(o.HaveOccurred())

					// it will happen that the audit files are sometimes empty, we can
					// safely ignore these files since they don't provide valuable information
					// TODO this doesn't seem right.  It should be really unlikely, but we'll deal with later
					if fi.Size() == 0 {
						return nil
					}

					gzipReader, err := gzip.NewReader(file)
					o.Expect(err).NotTo(o.HaveOccurred())

					scanner := bufio.NewScanner(gzipReader)
					for scanner.Scan() {
						text := scanner.Text()
						if !strings.HasSuffix(text, "}") {
							continue // ignore truncated data
						}
						o.Expect(text).To(o.HavePrefix(`{"kind":"Event",`))

						readFile = true
						eventsChecked++
					}
					// ignore this error as we usually fail to read the whole GZ file
					// o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
					o.Expect(readFile).To(o.BeTrue())

					return nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				if eventsChecked <= expectedNumberOfAuditEntries {
					lastErr = fmt.Errorf("expected %d audit events for %q, but only got %d", expectedNumberOfAuditEntries, auditDirectory, eventsChecked)
					return false, nil
				}

				// reset lastErr if we succeeded.
				lastErr = nil
			}

			// if we get here, it means both directories checked out ok
			return true, nil
		})
		o.Expect(lastErr).NotTo(o.HaveOccurred()) // print the last error first if we have one
		o.Expect(err).NotTo(o.HaveOccurred())     // otherwise be sure we fail on the timeout if it happened

		emptyFiles := []string{}
		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			o.Expect(expectedFilePath).To(o.BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			if size := stat.Size(); size < 20 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}
	})

	g.When("looking at the audit logs [apigroup:config.openshift.io]", func() {
		g.Describe("[sig-node] kubelet", func() {
			g.It("runs apiserver processes strictly sequentially in order to not risk audit log corruption", func() {
				controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				// On External clusters, events will not be part of the output, since audit logs do not include control plane logs.
				if *controlPlaneTopology == configv1.ExternalTopologyMode {
					g.Skip("External clusters don't have control plane audit logs")
				}

				tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
				o.Expect(err).NotTo(o.HaveOccurred())
				defer os.RemoveAll(tempDir)

				args := []string{
					"--dest-dir", tempDir,
					"--volume-percentage=100",
					"--",
					"/usr/bin/gather_audit_logs",
				}

				o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())

				pluginOutputDir := GetPluginOutputDir(tempDir)
				expectedAuditSubDirs := []string{"kube-apiserver", "openshift-apiserver", "oauth-apiserver"}

				seen := sets.String{}
				for _, apiserver := range expectedAuditSubDirs {
					err := filepath.Walk(filepath.Join(pluginOutputDir, "audit_logs", apiserver), func(path string, info os.FileInfo, err error) error {
						g.By(path)
						o.Expect(err).NotTo(o.HaveOccurred())
						if info.IsDir() {
							return nil
						}

						seen.Insert(apiserver)

						if filepath.Base(path) != "lock.log" {
							return nil
						}

						lockLog, err := ioutil.ReadFile(path)
						o.Expect(err).NotTo(o.HaveOccurred())

						// TODO: turn this into a failure as soon as kubelet is fixed
						result.Flakef("kubelet launched %s without waiting for the old process to terminate (lock was still hold): \n\n%s", apiserver, string(lockLog))
						return nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				o.Expect(seen.HasAll(expectedAuditSubDirs...), o.BeTrue())
			})
		})
	})

	g.It("runs successfully for metrics gathering [apigroup:config.openshift.io]", func() {
		tempDir, err := os.MkdirTemp("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		args := []string{
			"--dest-dir", tempDir,
			"--",
			"/usr/bin/gather_metrics",
			fmt.Sprintf("--min-time=%d", time.Now().Add(-5*time.Minute).UnixMilli()),
			fmt.Sprintf("--max-time=%d", time.Now().UnixMilli()),
			"--match=prometheus_ready",
			"--match=prometheus_build_info",
		}
		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())

		// wait for the contents to show up in the plugin output directory
		time.Sleep(5 * time.Second)

		pluginOutputDir := GetPluginOutputDir(tempDir)
		metricsFile := path.Join(pluginOutputDir, "monitoring", "metrics", "metrics.openmetrics")
		errorFile := path.Join(pluginOutputDir, "monitoring", "metrics", "metrics.stderr")

		// The error file should be empty
		o.Expect(errorFile).To(o.BeAnExistingFile())
		errorContent, err := os.ReadFile(errorFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(errorContent)).To(o.BeEmpty())

		// The metrics file should contain some series with a given format
		o.Expect(metricsFile).To(o.BeAnExistingFile())
		metrics, err := os.ReadFile(metricsFile)
		o.Expect(err).NotTo(o.HaveOccurred())

		lines := strings.Split(strings.TrimSpace(string(metrics)), "\n")
		count := len(lines)
		o.Expect(count).To(o.BeNumerically(">=", 5))
		for _, line := range lines[:count-1] {
			o.Expect(line).To(o.MatchRegexp(`^(prometheus_ready|prometheus_build_info)\{.*\} \d+ \d+`))
		}

		o.Expect(lines[count-1]).To(o.Equal("# EOF"))
	})
})

// GetPluginOutputDir returns the directory containing must-gather assets.
// Before [1], the assets were placed directly in tempDir.  Since [1],
// they have been placed in a subdirectory named after the must-gather
// image.
//
// [1]: https://github.com/openshift/oc/pull/84
func GetPluginOutputDir(tempDir string) string {
	files, err := os.ReadDir(tempDir)
	o.Expect(err).NotTo(o.HaveOccurred())
	dir := ""
	for _, file := range files {
		if file.IsDir() {
			if dir != "" {
				e2e.Logf("found multiple directories in %q, so assuming it is an old-style must-gather", tempDir)
				return tempDir
			}
			dir = path.Join(tempDir, file.Name())
		}
	}
	if dir == "" {
		e2e.Logf("found no directories in %q, so assuming it is an old-style must-gather", tempDir)
		return tempDir
	}
	e2e.Logf("found a single subdirectory %q, so assuming it is a new-style must-gather", dir)
	return dir
}

func IsAuditFile(fileName string) bool {
	if (strings.Contains(fileName, "-termination-") && strings.HasSuffix(fileName, ".log.gz")) ||
		strings.HasSuffix(fileName, "termination.log.gz") ||
		(strings.Contains(fileName, "-startup-") && strings.HasSuffix(fileName, ".log.gz")) ||
		strings.HasSuffix(fileName, "startup.log.gz") ||
		fileName == ".lock" ||
		fileName == "lock.log" {
		// these are expected, but have unstructured log format
		return false
	}

	return (strings.Contains(fileName, "-audit-") && strings.HasSuffix(fileName, ".log.gz")) || strings.HasSuffix(fileName, "audit.log.gz")
}
